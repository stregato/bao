package bao_ql

func (ds *BaoQL) MarshalYAML() (any, error) {
	return map[string]any{
		"id":      ds.s.Id,
		"group":   ds.group,
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
