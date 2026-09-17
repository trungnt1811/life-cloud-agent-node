package ingestion

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	domaintypes "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

type longProfile struct {
	name               string
	delimiter          rune
	decimalSeparator   string
	expectedDateFormat string
	dateLayout         string
	columns            columnNames
	testCodeColumn     string
	resultColumn       string
	unitColumn         string
	tests              map[string]testMapping // site test code → mapping
}

type longRow struct {
	rowNumber  int
	patientID  string
	specimenID string
	revision   int
	collected  string
	dateFormat string
	status     string
	testCode   string
	rawValue   string
	rawUnit    string
	provenance domaintypes.Provenance
}

func (p longProfile) ParseFile(path string) ([]domaintypes.NormalizedSpecimen, []domaintypes.Anomaly, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	csvReader := csv.NewReader(file)
	csvReader.Comma = p.delimiter
	csvReader.ReuseRecord = true

	headers, err := csvReader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("read header: %w", err)
	}
	headers = cloneRecord(headers)
	if err := requireColumns(headers,
		p.columns.PatientID, p.columns.SpecimenID, p.columns.Date, p.columns.DateFormat,
		p.columns.Revision, p.columns.Status, p.testCodeColumn, p.resultColumn, p.unitColumn,
		"source_dataset", "source_file", "source_row_number", "source_record_id",
	); err != nil {
		return nil, nil, err
	}

	grouped := map[string][]longRow{}
	var anomalies []domaintypes.Anomaly
	rowNumber := 0
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("read row: %w", err)
		}
		rowNumber++
		row := rowMap(headers, record)

		patientID := strings.TrimSpace(row[p.columns.PatientID])
		if patientID == "" {
			anomalies = append(anomalies, anomaly(path, rowNumber, "missing_patient_id"))
			continue
		}
		if reason := validateFinalRevision(row[p.columns.Status], row[p.columns.Revision]); reason != "" {
			anomalies = append(anomalies, anomaly(path, rowNumber, reason))
			continue
		}
		revision, err := strconv.Atoi(strings.TrimSpace(row[p.columns.Revision]))
		if err != nil {
			anomalies = append(anomalies, anomaly(path, rowNumber, "invalid_revision_or_status"))
			continue
		}
		specimenID := strings.TrimSpace(row[p.columns.SpecimenID])
		if specimenID == "" {
			anomalies = append(anomalies, anomaly(path, rowNumber, "missing_specimen_id"))
			continue
		}

		key := patientID + "\x00" + specimenID
		grouped[key] = append(grouped[key], longRow{
			rowNumber:  rowNumber,
			patientID:  patientID,
			specimenID: specimenID,
			revision:   revision,
			collected:  row[p.columns.Date],
			dateFormat: row[p.columns.DateFormat],
			status:     row[p.columns.Status],
			testCode:   strings.TrimSpace(row[p.testCodeColumn]),
			rawValue:   row[p.resultColumn],
			rawUnit:    row[p.unitColumn],
			provenance: provenanceFromRow(row),
		})
	}

	var specimens []domaintypes.NormalizedSpecimen
	for _, rows := range grouped {
		specimen, groupAnomalies := p.normalizeGroup(path, rows)
		anomalies = append(anomalies, groupAnomalies...)
		if specimen != nil {
			specimens = append(specimens, *specimen)
		}
	}
	return specimens, anomalies, nil
}

func (p longProfile) normalizeGroup(path string, rows []longRow) (*domaintypes.NormalizedSpecimen, []domaintypes.Anomaly) {
	var anomalies []domaintypes.Anomaly
	if len(rows) == 0 {
		return nil, nil
	}

	// Use the first row's date/provenance; if the group has an ambiguous or
	// invalid date, quarantine the whole specimen (life-cloud policy).
	head := rows[0]
	collectedAt, reason := parseCollectedAt(head.collected, head.dateFormat, p.expectedDateFormat, p.dateLayout)
	if reason != "" {
		for _, row := range rows {
			anomalies = append(anomalies, anomaly(path, row.rowNumber, reason))
		}
		return nil, anomalies
	}

	type candidate struct {
		row         longRow
		observation domaintypes.NormalizedObservation
		revision    int
	}
	best := map[string]candidate{}
	for _, row := range rows {
		mapping, ok := p.tests[row.testCode]
		if !ok {
			anomalies = append(anomalies, anomaly(path, row.rowNumber, "unknown_test:"+row.testCode))
			continue
		}
		observation, obsReason := normalizeMeasurement(row.rawValue, row.rawUnit, p.decimalSeparator, mapping)
		if obsReason != "" {
			anomalies = append(anomalies, anomaly(path, row.rowNumber, obsReason+":"+mapping.Code))
			continue
		}
		prev, exists := best[mapping.Code]
		if !exists || row.revision > prev.revision {
			best[mapping.Code] = candidate{row: row, observation: observation, revision: row.revision}
		}
	}
	if len(best) == 0 {
		anomalies = append(anomalies, anomaly(path, head.rowNumber, "no_accepted_observations"))
		return nil, anomalies
	}

	observations := make([]domaintypes.NormalizedObservation, 0, len(best))
	for _, item := range best {
		observations = append(observations, item.observation)
	}

	return &domaintypes.NormalizedSpecimen{
		ExternalPatientID: head.patientID,
		CollectedAt:       collectedAt,
		Observations:      observations,
		Provenance:        head.provenance,
	}, anomalies
}
