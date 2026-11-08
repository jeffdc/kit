package storage

import (
	"database/sql"
	"encoding/json"

	"forage/internal/model"
)

// ApplyOp applies one client op. It is idempotent: an op_id that was already
// applied returns the stored result without changing anything, so clients
// can safely resend a batch whose response they never received.
//
// Problems with the op itself come back as a "rejected" result with a nil
// error. A non-nil error means the server failed and the op should be retried.
func (s *Store) ApplyOp(op model.Op) (model.OpResult, error) {
	res := model.OpResult{OpID: op.OpID}
	if op.OpID == "" || len(op.OpID) > 64 {
		return rejected(res, "op_id is required (at most 64 characters)"), nil
	}

	var stored string
	err := s.db.QueryRow("SELECT result FROM applied_ops WHERE op_id = ?", op.OpID).Scan(&stored)
	if err == nil {
		var prev model.OpResult
		if err := json.Unmarshal([]byte(stored), &prev); err != nil {
			return res, err
		}
		return prev, nil
	}
	if err != sql.ErrNoRows {
		return res, err
	}

	_, err = s.write(func(tx *sql.Tx, rev int64, now string) error {
		if err := applyOpTx(tx, rev, now, op); err != nil {
			return err
		}
		res.Status, res.Rev = "ok", rev
		data, err := json.Marshal(res)
		if err != nil {
			return err
		}
		_, err = tx.Exec("INSERT INTO applied_ops (op_id, result, applied_at) VALUES (?, ?, ?)", op.OpID, string(data), now)
		return err
	})
	if err != nil {
		if IsRejection(err) {
			return rejected(model.OpResult{OpID: op.OpID}, err.Error()), nil
		}
		return model.OpResult{OpID: op.OpID}, err
	}
	return res, nil
}

func rejected(res model.OpResult, msg string) model.OpResult {
	res.Status, res.Error, res.Rev = "rejected", msg, 0
	return res
}

func applyOpTx(tx *sql.Tx, rev int64, now string, op model.Op) error {
	var err error
	switch op.Type {
	case "book.create":
		if op.Book == nil {
			return invalid("book.create needs a book")
		}
		_, err = createBookTx(tx, rev, now, op.Book)
	case "book.update":
		_, err = updateBookTx(tx, rev, now, op.ID, fieldStrings(op.Fields))
	case "book.delete":
		err = deleteBookTx(tx, rev, now, op.ID)
	case "read.create":
		if op.Read == nil {
			return invalid("read.create needs a read")
		}
		_, err = createReadTx(tx, rev, now, *op.Read)
	case "read.update":
		_, err = updateReadTx(tx, rev, now, op.ID, fieldStrings(op.Fields))
	case "read.delete":
		err = deleteReadTx(tx, rev, now, op.ID)
	default:
		return invalid("unknown op type %q", op.Type)
	}
	return err
}

func fieldStrings(fields map[string]any) map[string]string {
	out := make(map[string]string, len(fields))
	for k, v := range fields {
		out[k] = FieldString(v)
	}
	return out
}
