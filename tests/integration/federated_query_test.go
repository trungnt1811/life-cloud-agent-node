package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/lifenetwork-ai/life-cloud-agent-node/cmd/app"
	"github.com/lifenetwork-ai/life-cloud-agent-node/conf"
	"github.com/lifenetwork-ai/life-cloud-agent-node/constants"
	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	fedclient "github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/client"
	federatedwire "github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/wire"
	repotestsupport "github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/testsupport"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/di"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	domainrepos "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
	domaintypes "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/interfaces"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/runtimeconfig"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/testsupport/controlcenterstub"
)

// Phase 9: an end-to-end proof that the real Phase 7 client, the Phase 8
// stub control center, the full HTTP admin surface, and D3/D4/D5 all work
// together - not just individually, as every earlier phase already proved
// in isolation.

const (
	e2eAdminUser = "admin"
	e2eAdminPass = "secret"
)

// federatedHarness wires one node end-to-end: a real Postgres-backed D3/D4/D5,
// the production HTTP router (admin auth included), and a real NodeClient
// dialing a Phase 8 stub over loopback TCP.
type federatedHarness struct {
	db         *gorm.DB
	repos      *di.Repos
	httpServer *httptest.Server
	stub       *controlcenterstub.Server
	cancel     context.CancelFunc
}

// setupFederatedHarness boots one node. cohortCounter, when non-nil,
// overrides the client's counter (used to deterministically interrupt a job
// mid-chunk); nil uses the same real, checkpointed use case production
// wiring uses.
func setupFederatedHarness(t *testing.T, cohortCounter interfaces.CohortCountUseCase) *federatedHarness {
	t.Helper()

	db := repotestsupport.OpenPostgresDB(t)
	repotestsupport.RequirePostgresDialect(t, db)
	repotestsupport.ResetPostgresRepositoryTables(t, db)

	config := &conf.Configuration{
		AppName:            "life-cloud-agent-node-e2e",
		Env:                "TEST",
		LogLevel:           "info",
		CacheType:          constants.CacheTypeInMemory,
		AdminBasicAuthUser: e2eAdminUser,
		AdminBasicAuthPass: e2eAdminPass,
	}
	appLogger := logger.GetLogger()
	advisories := fedclient.NewAdvisoryStore()

	ctx, cancel := context.WithCancel(context.Background())
	router := app.SetupRouter(ctx, db, config, advisories, appLogger)
	httpServer := httptest.NewServer(router)
	t.Cleanup(httpServer.Close)

	stub := controlcenterstub.New()
	stubAddr := stub.Start(t)

	repos := di.InitializeRepos(ctx, db, appLogger, runtimeconfig.ModuleConfigsFromConfiguration(config))
	if cohortCounter == nil {
		cohortCounter = usecases.NewCohortCountUseCase(repos.PatientRegistryRepo, repos.JobProgressRepo, 0, appLogger)
	}

	nodeClient := fedclient.NewNodeClient(fedclient.Config{
		Address:            stubAddr,
		NodeID:             "e2e-node",
		AgentVersion:       "e2e-test",
		QuerySchemaVersion: 1,
		TLS:                fedclient.TLSConfig{Insecure: true},
		HeartbeatInterval:  200 * time.Millisecond,
		ConnectTimeout:     2 * time.Second,
		InitialBackoff:     20 * time.Millisecond,
		MaxBackoff:         100 * time.Millisecond,
	}, fedclient.Dependencies{
		EnabledQueryFieldRepo: repos.EnabledQueryFieldRepo,
		CohortCounter:         cohortCounter,
		SuppressionThreshold:  1, // hides nothing, for deterministic assertions
		Advisories:            advisories,
	}, appLogger, nil)

	go func() { _ = nodeClient.Run(ctx) }()
	t.Cleanup(cancel)

	h := &federatedHarness{db: db, repos: repos, httpServer: httpServer, stub: stub, cancel: cancel}
	h.waitForRegister(t)
	return h
}

func (h *federatedHarness) waitForRegister(t *testing.T) {
	t.Helper()
	select {
	case <-h.stub.Registers():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the node to register with the stub")
	}
}

func (h *federatedHarness) getStatus(t *testing.T) map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, h.httpServer.URL+"/admin/status", nil)
	require.NoError(t, err)
	req.SetBasicAuth(e2eAdminUser, e2eAdminPass)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	return payload
}

type labObservation struct {
	value    string
	censored bool
}

// seedPatientWithSpecimen creates one patient with one specimen and its lab
// observations, mirroring a real ingested HIS/LIS export (Phase 1-2).
func seedPatientWithSpecimen(
	t *testing.T,
	repo domainrepos.PatientRegistryRepository,
	externalPatientID, externalSpecimenID string,
	collectedAt time.Time,
	observations map[string]labObservation,
) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()

	patient, err := repo.GetOrCreatePatient(ctx, externalPatientID)
	require.NoError(t, err)

	specimenID := uuid.New()
	specimen := entities.NewSpecimen(entities.NewSpecimenParams{
		ID:                 specimenID,
		PatientID:          patient.ID(),
		ExternalSpecimenID: externalSpecimenID,
		CollectedAt:        collectedAt,
		SourceDataset:      "E2E",
		SourceFile:         "e2e.csv",
		SourceRecordID:     externalSpecimenID,
		SourceRowNumber:    1,
		Now:                now,
	})

	obs := make([]*entities.LabObservation, 0, len(observations))
	for code, o := range observations {
		obs = append(obs, entities.NewLabObservation(entities.NewLabObservationParams{
			ID:         uuid.New(),
			SpecimenID: specimenID,
			FieldCode:  code,
			Value:      o.value,
			Censored:   o.censored,
			RawValue:   o.value,
			RawUnit:    "u",
			Revision:   1,
			Now:        now,
		}))
	}
	require.NoError(t, repo.SaveSpecimen(ctx, specimen, obs))
}

func enableFields(t *testing.T, repo domainrepos.EnabledQueryFieldRepository, codes ...string) {
	t.Helper()
	now := time.Now().UTC()
	for _, code := range codes {
		require.NoError(t, repo.Upsert(context.Background(), entities.NewEnabledQueryField(code, true, "e2e-test", now)))
	}
}

// demoRulePanels is the field set the compiled demo QueryTask needs enabled
// (docs/product/federated-query-flow.md / decision 0004's demo fixture).
var demoRulePanels = []string{"HB", "MCV", "MCH", "RBC", "HBA0", "HBA2", "HBF"}

func inRangeDate(t *testing.T) time.Time {
	t.Helper()
	return time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
}

func matchingCBC() map[string]labObservation {
	return map[string]labObservation{
		"HB":  {value: "10.8"},
		"MCV": {value: "70"}, // < 80
		"MCH": {value: "22"}, // < 27
		"RBC": {value: "5.1"},
	}
}

func nonMatchingCBC() map[string]labObservation {
	return map[string]labObservation{
		"HB":  {value: "13.2"},
		"MCV": {value: "88"}, // not < 80
		"MCH": {value: "29"}, // not < 27
		"RBC": {value: "4.5"},
	}
}

func hplcPanel(censoredHBF bool) map[string]labObservation {
	return map[string]labObservation{
		"HBA0": {value: "96.5"},
		"HBA2": {value: "2.8"},
		"HBF":  {value: "0.1", censored: censoredHBF},
	}
}

func merge(maps ...map[string]labObservation) map[string]labObservation {
	out := make(map[string]labObservation)
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

func TestFederatedQuery_ValidQueryReturnsMatchingCount(t *testing.T) {
	h := setupFederatedHarness(t, nil)
	enableFields(t, h.repos.EnabledQueryFieldRepo, demoRulePanels...)

	day := inRangeDate(t)
	seedPatientWithSpecimen(t, h.repos.PatientRegistryRepo, "MATCH-1", "S-1", day,
		merge(matchingCBC(), hplcPanel(false)))
	seedPatientWithSpecimen(t, h.repos.PatientRegistryRepo, "NO-MATCH-1", "S-2", day,
		merge(nonMatchingCBC(), hplcPanel(false)))

	task, err := federatedwire.CompileDemoQueryTaskFromFixture()
	require.NoError(t, err)
	task.JobId = "e2e-valid-query"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, h.stub.SendQueryTask(ctx, task))

	select {
	case result := <-h.stub.QueryResults():
		require.Equal(t, "e2e-valid-query", result.GetJobId())
		require.Equal(t, nodev1.QueryResultStatus_QUERY_RESULT_STATUS_OK, result.GetStatus())
		require.Equal(t, uint64(1), result.GetMatchingCount())
		require.False(t, result.GetSuppressed())
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for QueryResult")
	}
}

func TestFederatedQuery_DisabledFieldIsRejected(t *testing.T) {
	h := setupFederatedHarness(t, nil)
	// Enable every demo-rule field except HBF, so the HPLC panel references a
	// field this node has not opted into sharing (process 5.0 / decision 0002).
	enabled := make([]string, 0, len(demoRulePanels))
	for _, code := range demoRulePanels {
		if code != "HBF" {
			enabled = append(enabled, code)
		}
	}
	enableFields(t, h.repos.EnabledQueryFieldRepo, enabled...)

	day := inRangeDate(t)
	seedPatientWithSpecimen(t, h.repos.PatientRegistryRepo, "MATCH-1", "S-1", day,
		merge(matchingCBC(), hplcPanel(false)))

	task, err := federatedwire.CompileDemoQueryTaskFromFixture()
	require.NoError(t, err)
	task.JobId = "e2e-disabled-field"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, h.stub.SendQueryTask(ctx, task))

	select {
	case result := <-h.stub.QueryResults():
		require.Equal(t, "e2e-disabled-field", result.GetJobId())
		require.Equal(t, nodev1.QueryResultStatus_QUERY_RESULT_STATUS_REJECTED_INVALID_QUERY, result.GetStatus())
		require.Contains(t, result.GetReason(), "HBF")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for QueryResult")
	}
}

func TestFederatedQuery_UpdateAdvisoryReflectedInStatus(t *testing.T) {
	h := setupFederatedHarness(t, nil)

	status := h.getStatus(t)
	require.NotContains(t, status, "advisory", "no advisory received yet")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, h.stub.SendUpdateAdvisory(ctx, &nodev1.UpdateAdvisory{
		Version:      "v9.9.9",
		ChangelogUrl: "https://example.invalid/changelog",
		Severity:     nodev1.AdvisorySeverity_ADVISORY_SEVERITY_CRITICAL,
	}))

	require.Eventually(t, func() bool {
		status := h.getStatus(t)
		advisory, ok := status["advisory"].(map[string]any)
		return ok && advisory["version"] == "v9.9.9"
	}, 5*time.Second, 50*time.Millisecond, "GET /admin/status must reflect the received advisory")

	status = h.getStatus(t)
	advisory := status["advisory"].(map[string]any)
	require.Equal(t, "https://example.invalid/changelog", advisory["changelog_url"])
	require.Equal(t, "ADVISORY_SEVERITY_CRITICAL", advisory["severity"])
}

// dropAfterNthChunk wraps a real PatientRegistryRepository and, on its Nth
// CountMatchingCohortChunk call, triggers onTrigger() and then blocks until
// ctx is canceled before returning. Because the trigger fires only after the
// real chunk read has already completed, that chunk's data is never in
// question; blocking until cancellation guarantees the use case's subsequent
// checkpoint Save() (same ctx) observes the cancellation deterministically -
// database/sql aborts a Context call immediately when ctx is already done -
// instead of racing an async, cross-goroutine network disconnect.
type dropAfterNthChunk struct {
	domainrepos.PatientRegistryRepository
	calls         atomic.Int64
	dropAfterCall int64
	triggered     atomic.Bool
	onTrigger     func()
}

func (d *dropAfterNthChunk) CountMatchingCohortChunk(
	ctx context.Context,
	criteria domaintypes.CohortCriteria,
	afterPatientID uuid.UUID,
	limit int,
) (domaintypes.CohortChunkResult, error) {
	result, err := d.PatientRegistryRepository.CountMatchingCohortChunk(ctx, criteria, afterPatientID, limit)
	if n := d.calls.Add(1); n == d.dropAfterCall && d.triggered.CompareAndSwap(false, true) {
		d.onTrigger()
		<-ctx.Done()
	}
	return result, err
}

func TestFederatedQuery_ResumesAfterDroppedConnection(t *testing.T) {
	db := repotestsupport.OpenPostgresDB(t)
	repotestsupport.RequirePostgresDialect(t, db)
	repotestsupport.ResetPostgresRepositoryTables(t, db)

	config := &conf.Configuration{
		AppName: "life-cloud-agent-node-e2e", Env: "TEST", LogLevel: "info",
		CacheType:          constants.CacheTypeInMemory,
		AdminBasicAuthUser: e2eAdminUser, AdminBasicAuthPass: e2eAdminPass,
	}
	appLogger := logger.GetLogger()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	repos := di.InitializeRepos(ctx, db, appLogger, runtimeconfig.ModuleConfigsFromConfiguration(config))
	enableFields(t, repos.EnabledQueryFieldRepo, demoRulePanels...)

	// Three candidate patients with chunk size 1 forces three separate chunk
	// reads, giving a real "mid-task" point to interrupt between chunk 2 and 3.
	day := inRangeDate(t)
	seedPatientWithSpecimen(t, repos.PatientRegistryRepo, "MATCH-A", "S-A", day, merge(matchingCBC(), hplcPanel(false)))
	seedPatientWithSpecimen(t, repos.PatientRegistryRepo, "NO-MATCH", "S-B", day, merge(nonMatchingCBC(), hplcPanel(false)))
	seedPatientWithSpecimen(t, repos.PatientRegistryRepo, "MATCH-C", "S-C", day, merge(matchingCBC(), hplcPanel(false)))

	stub := controlcenterstub.New()
	stubAddr := stub.Start(t)

	decorated := &dropAfterNthChunk{PatientRegistryRepository: repos.PatientRegistryRepo, dropAfterCall: 2, onTrigger: stub.DropConnection}
	cohortCounter := usecases.NewCohortCountUseCase(decorated, repos.JobProgressRepo, 1, appLogger)

	nodeClient := fedclient.NewNodeClient(fedclient.Config{
		Address: stubAddr, NodeID: "e2e-node", AgentVersion: "e2e-test", QuerySchemaVersion: 1,
		TLS:               fedclient.TLSConfig{Insecure: true},
		HeartbeatInterval: 200 * time.Millisecond, ConnectTimeout: 2 * time.Second,
		InitialBackoff: 20 * time.Millisecond, MaxBackoff: 100 * time.Millisecond,
	}, fedclient.Dependencies{
		EnabledQueryFieldRepo: repos.EnabledQueryFieldRepo,
		CohortCounter:         cohortCounter,
		SuppressionThreshold:  1,
	}, appLogger, nil)

	go func() { _ = nodeClient.Run(ctx) }()

	select {
	case <-stub.Registers():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the first Register")
	}

	task, err := federatedwire.CompileDemoQueryTaskFromFixture()
	require.NoError(t, err)
	task.JobId = "e2e-resume-after-drop"

	sendCtx, sendCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer sendCancel()
	require.NoError(t, stub.SendQueryTask(sendCtx, task))

	// The interrupted first attempt must send nothing; only the reconnect and
	// resend below produce a result.
	select {
	case result := <-stub.QueryResults():
		t.Fatalf("unexpected QueryResult from the interrupted attempt: %+v", result)
	case <-time.After(300 * time.Millisecond):
	}

	select {
	case <-stub.Registers():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the reconnect Register")
	}

	resendCtx, resendCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer resendCancel()
	require.NoError(t, stub.SendQueryTask(resendCtx, task))

	select {
	case result := <-stub.QueryResults():
		require.Equal(t, "e2e-resume-after-drop", result.GetJobId())
		require.Equal(t, nodev1.QueryResultStatus_QUERY_RESULT_STATUS_OK, result.GetStatus())
		require.Equal(t, uint64(2), result.GetMatchingCount(), "MATCH-A and MATCH-C; the interruption must not lose or double-count a chunk")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the resumed QueryResult")
	}

	progress, err := repos.JobProgressRepo.Get(context.Background(), "e2e-resume-after-drop")
	require.NoError(t, err)
	require.True(t, progress.Done())
	require.Equal(t, uint64(2), progress.RunningCount())
}
