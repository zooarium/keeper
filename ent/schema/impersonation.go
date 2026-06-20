package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ImpersonationSession holds the schema definition for an impersonation
// session: an audit record of a sysadmin acting as another user on a single
// downstream service. The minted JWT carries the impersonated user's identity;
// this row records who is really driving (impersonator), the scope (app /
// division / target / audience), and the lifecycle (status, expiry, revocation)
// so a session can be killed server-side before its token expires.
type ImpersonationSession struct {
	ent.Schema
}

// Annotations of the ImpersonationSession.
func (ImpersonationSession) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "kpr_impersonation_session"},
	}
}

// Fields of the ImpersonationSession.
func (ImpersonationSession) Fields() []ent.Field {
	return []ent.Field{
		// session_id is the opaque, unguessable identifier carried in the token
		// (sid claim). Unique so revocation and status lookups resolve to one row.
		field.String("session_id").Unique(),
		field.Int("app_id"),
		field.Int("division_id"),
		// impersonator_user_id is the acting sysadmin; target_user_id is the
		// user being impersonated (whose identity the token carries).
		field.Int("impersonator_user_id"),
		field.Int("target_user_id"),
		// audience is the single downstream service key this session is scoped to.
		field.String("audience"),
		// read_only mirrors the token's imp_ro claim.
		field.Bool("read_only").Default(false),
		field.String("reason").Optional(),
		// status: 1 = active, 0 = revoked.
		field.Int8("status").Default(1),
		field.Time("created_at").Default(time.Now),
		field.Time("expires_at"),
		field.Time("revoked_at").Optional().Nillable(),
	}
}

// Indexes of the ImpersonationSession.
func (ImpersonationSession) Indexes() []ent.Index {
	return []ent.Index{
		// app_id scopes the admin list query (mirrors the other entities).
		index.Fields("app_id"),
	}
}
