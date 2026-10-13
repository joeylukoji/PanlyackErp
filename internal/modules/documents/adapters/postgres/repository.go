package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"panlyackerp/internal/modules/documents/domain"
	"panlyackerp/internal/modules/documents/ports"
	"panlyackerp/internal/platform/db"
)

type Repository struct{ DB db.Store }
type File = ports.File

func (r Repository) Create(ctx context.Context, i domain.Invoice, f File) error {
	data, err := json.Marshal(i)
	if err != nil {
		return err
	}
	_, err = r.DB.Conn(ctx).Exec(ctx, `INSERT INTO documents.invoices(organization_id,id,document_id,status,data,storage_key,mime_type,sha256) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, i.OrganizationID, i.ID, i.DocumentID, i.Status, data, f.Key, f.MIME, f.Hash)
	return err
}

func (r Repository) Get(ctx context.Context, org, id string, lock bool) (domain.Invoice, error) {
	query := `SELECT data,status,version FROM documents.invoices WHERE organization_id=$1 AND id=$2`
	if lock {
		query += " FOR UPDATE"
	}
	var data []byte
	var status string
	var version int
	err := r.DB.Conn(ctx).QueryRow(ctx, query, org, id).Scan(&data, &status, &version)
	i := domain.Invoice{}
	if err != nil {
		return i, err
	}
	if err := json.Unmarshal(data, &i); err != nil {
		return i, err
	}
	i.Status = status
	i.Version = version
	return i, nil
}

func (r Repository) Save(ctx context.Context, i domain.Invoice) error {
	data, err := json.Marshal(i)
	if err != nil {
		return err
	}
	tag, err := r.DB.Conn(ctx).Exec(ctx, `UPDATE documents.invoices SET data=$3,status=$4,version=version+1 WHERE organization_id=$1 AND id=$2 AND version=$5 AND status<>'POSTED'`, i.OrganizationID, i.ID, data, i.Status, i.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("invoice changed; reload before saving")
	}
	return nil
}

func (r Repository) List(ctx context.Context, org string) ([]domain.Invoice, error) {
	rows, err := r.DB.Conn(ctx).Query(ctx, `SELECT data,status,version FROM documents.invoices WHERE organization_id=$1 ORDER BY created_at DESC LIMIT 100`, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.Invoice{}
	for rows.Next() {
		var data []byte
		var status string
		var version int
		if err := rows.Scan(&data, &status, &version); err != nil {
			return nil, err
		}
		i := domain.Invoice{}
		if err := json.Unmarshal(data, &i); err != nil {
			return nil, err
		}
		i.Status = status
		i.Version = version
		result = append(result, i)
	}
	return result, rows.Err()
}

func (r Repository) File(ctx context.Context, org, id string) (File, error) {
	f := File{}
	err := r.DB.Conn(ctx).QueryRow(ctx, `SELECT storage_key,mime_type,sha256 FROM documents.invoices WHERE organization_id=$1 AND id=$2`, org, id).Scan(&f.Key, &f.MIME, &f.Hash)
	return f, err
}
