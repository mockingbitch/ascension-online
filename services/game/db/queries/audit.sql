-- name: InsertAdminAudit :exec
INSERT INTO admin_audit_logs (admin_user_id, action, target_type, target_id, before, after, reason)
VALUES ($1, $2, $3, $4, $5, $6, $7);
