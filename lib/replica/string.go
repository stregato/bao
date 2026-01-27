package replica

func (ds *Replica) MarshalYAML() (any, error) {
	return map[string]any{
		"id":      ds.vault.ID,
		"lastId":  ds.lastId,
		"updates": ds.transaction,
	}, nil
}

func (transaction *transaction) MarshalYAML() (any, error) {
	return map[string]any{
		"id":      transaction.Id,
		"version": transaction.Version,
		"tm":      transaction.Tm,
		"updates": transaction.Updates,
	}, nil
}
