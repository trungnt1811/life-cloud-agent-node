package entities

func (b GovernedHospitalBinding) Validate(protected bool) error {
	if !validGovernedUUID(b.TenantID) || !validGovernedUUID(b.QueryID) || !validGovernedUUID(b.JobID) || !validGovernedUUID(b.EventID) || !governedHospitalIdentifier(b.NodeID) || !governedHospitalIdentifier(b.SessionEpoch) || !hospitalPermitIDPattern.MatchString(b.PermitID) || b.PermitVersion < 1 || b.LocalPolicyVersion < 1 || b.AcceptanceRevision < 1 || !hospitalHash(b.DefinitionHash) || !hospitalHash(b.PermitHash) || !hospitalHash(b.LocalPolicyHash) || !hospitalHash(b.AuthorizationSnapshotHash) {
		return ErrGovernedHospitalJobInvalid
	}
	if protected && !hospitalHash(b.ProtectedPayloadDigest) || !protected && b.ProtectedPayloadDigest != "" {
		return ErrGovernedHospitalJobInvalid
	}
	return nil
}

func (r GovernedHospitalReceipt) ValidateProof() error {
	if !validGovernedUUID(r.ID) || !validGovernedUUID(r.JobID) || !validGovernedUUID(r.EventID) || !hospitalHash(r.ProtectedPayloadDigest) || r.CommittedAt.IsZero() || !validHospitalReceiptOutcome(r) {
		return ErrGovernedHospitalJobInvalid
	}
	return nil
}
