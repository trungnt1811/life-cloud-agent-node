package ingestion

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	domaintypes "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

type wideProfile struct {
	name               string
	delimiter          rune
	stripBOM           bool
	decimalSeparator   string
	expectedDateFormat string
	dateLayout         string
	columns            columnNames
	tests              map[string]testMapping // export column label → mapping
}

func (p wideProfile) ParseFile(path string) ([]domaintypes.NormalizedSpecimen, []domaintypes.Anomaly, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	var reader io.Reader = file
	if p.stripBOM {
		reader = stripUTF8BOM(file)
	}

	csvReader := csv.NewReader(reader)
	csvReader.Comma = p.delimiter
	csvReader.ReuseRecord = true

	headers, err := csvReader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("read header: %w", err)
	}
	headers = cloneRecord(headers)
	if err := requireColumns(headers,
		p.columns.PatientID, p.columns.SpecimenID, p.columns.Date, p.columns.DateFormat,
		p.columns.Revision, p.columns.Status,
		"source_dataset", "source_file", "source_row_number", "source_record_id",
	); err != nil {
		return nil, nil, err
	}

	var (
		specimens []domaintypes.NormalizedSpecimen
		anomalies []domaintypes.Anomaly
		rowNumber int
	)
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
		specimenID := strings.TrimSpace(row[p.columns.SpecimenID])
		if specimenID == "" {
			anomalies = append(anomalies, anomaly(path, rowNumber, "missing_specimen_id"))
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
		collectedAt, reason := parseCollectedAt(
			row[p.columns.Date],
			row[p.columns.DateFormat],
			p.expectedDateFormat,
			p.dateLayout,
		)
		if reason != "" {
			anomalies = append(anomalies, anomaly(path, rowNumber, reason))
			continue
		}

		observations := make([]domaintypes.NormalizedObservation, 0, len(p.tests))
		for label, mapping := range p.tests {
			rawValue := row[label]
			rawUnit := row[label+"_unit"]
			if strings.TrimSpace(rawValue) == "" && strings.TrimSpace(rawUnit) == "" {
				continue
			}
			observation, obsReason := normalizeMeasurement(rawValue, rawUnit, p.decimalSeparator, mapping)
			if obsReason != "" {
				anomalies = append(anomalies, anomaly(path, rowNumber, obsReason+":"+mapping.Code))
				continue
			}
			observation.Revision = revision
			observations = append(observations, observation)
		}
		if len(observations) == 0 {
			anomalies = append(anomalies, anomaly(path, rowNumber, "no_accepted_observations"))
			continue
		}

		specimens = append(specimens, domaintypes.NormalizedSpecimen{
			ExternalPatientID:  patientID,
			ExternalSpecimenID: specimenID,
			CollectedAt:        collectedAt,
			Observations:       observations,
			Provenance:         provenanceFromRow(row),
		})
	}

	return specimens, anomalies, nil
}

func rowMap(headers, record []string) map[string]string {
	out := make(map[string]string, len(headers))
	for i, header := range headers {
		if i >= len(record) {
			out[header] = ""
			continue
		}
		out[header] = record[i]
	}
	return out
}

func cloneRecord(record []string) []string {
	out := make([]string, len(record))
	copy(out, record)
	return out
}

func stripUTF8BOM(r io.Reader) io.Reader {
	return &bomStrippingReader{r: r}
}

type bomStrippingReader struct {
	r       io.Reader
	checked bool
}

func (b *bomStrippingReader) Read(p []byte) (int, error) {
	if !b.checked {
		b.checked = true
		var bom [3]byte
		n, err := io.ReadFull(b.r, bom[:])
		if n == 3 && bom == [3]byte{0xEF, 0xBB, 0xBF} {
			return b.r.Read(p)
		}
		if n > 0 {
			b.r = io.MultiReader(bytes.NewReader(bom[:n]), b.r)
		}
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return 0, err
		}
	}
	return b.r.Read(p)
}
