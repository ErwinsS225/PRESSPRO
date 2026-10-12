package order

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"

	"github.com/jackc/pgx/v5"
	"pressing-os/backend/internal/auth"
)

var mutationKeyRE = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

func validMutationKey(key string) error {
	if key != "" && !mutationKeyRE.MatchString(key) {
		return ErrInvalidIdempotencyKey
	}
	return nil
}
func paymentActor(ctx context.Context) string {
	if c := auth.GetUserFromContext(ctx); c != nil {
		return c.UserID
	}
	return ""
}
func mutationHash(ctx context.Context, req interface{}) string {
	data, _ := json.Marshal(struct {
		Actor   string
		Request interface{}
	}{paymentActor(ctx), req})
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// La clé et la réponse sont écrites dans la même transaction que l'encaissement.
// Aucune expiration automatique : un ancien paiement ne doit pas être rejoué.
func replayTx(ctx context.Context, tx pgx.Tx, tenant, op, key, hash string) (*Order, error) {
	if key == "" {
		return nil, nil
	}
	lock, _ := json.Marshal([]string{tenant, op, key})
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, string(lock)); err != nil {
		return nil, err
	}
	var stored string
	var data []byte
	err := tx.QueryRow(ctx, `SELECT request_hash,response FROM order_mutations WHERE tenant_id=$1 AND operation=$2 AND mutation_key=$3`, tenant, op, key).Scan(&stored, &data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if stored != hash {
		return nil, ErrIdempotencyConflict
	}
	var o Order
	err = json.Unmarshal(data, &o)
	return &o, err
}
func saveReplayTx(ctx context.Context, tx pgx.Tx, tenant, op, key, hash string, o *Order) error {
	if key == "" {
		return nil
	}
	data, err := json.Marshal(o)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO order_mutations(tenant_id,operation,mutation_key,request_hash,response) VALUES($1,$2,$3,$4,$5)`, tenant, op, key, hash, data)
	return err
}

type memoryMutation struct {
	hash     string
	response []byte
}

func memoryKey(tenant, op, key string) string {
	b, _ := json.Marshal([]string{tenant, op, key})
	return string(b)
}

// Appels sous s.mu, après sérialisation des créations s.creation si nécessaire.
func (s *Service) replayMemory(tenant, op, key, hash string) (*Order, error) {
	if key == "" {
		return nil, nil
	}
	if m, ok := s.mutations[memoryKey(tenant, op, key)]; ok {
		if m.hash != hash {
			return nil, ErrIdempotencyConflict
		}
		var o Order
		err := json.Unmarshal(m.response, &o)
		return &o, err
	}
	return nil, nil
}
func (s *Service) saveMemory(tenant, op, key, hash string, o *Order) {
	if key == "" {
		return
	}
	data, _ := json.Marshal(o)
	s.mutations[memoryKey(tenant, op, key)] = memoryMutation{hash, data}
}
func cloneOrder(o *Order) *Order {
	b, _ := json.Marshal(o)
	var c Order
	_ = json.Unmarshal(b, &c)
	return &c
}
