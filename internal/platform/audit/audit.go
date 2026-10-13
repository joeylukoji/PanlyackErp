package audit

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"panlyackerp/internal/platform/db"
)

type Log struct{ DB db.Store }

func (l Log) Append(ctx context.Context, org, actor, action, id string, details any) error {
	data, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = l.DB.Conn(ctx).Exec(ctx, `INSERT INTO platform.audit_log(id,organization_id,actor_id,action,entity_id,details) VALUES($1,$2,$3,$4,$5,$6)`, uuid.NewString(), org, actor, action, id, data)
	return err
}
