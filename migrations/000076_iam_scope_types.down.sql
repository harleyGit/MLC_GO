USE HG_MLC_DB;
ALTER TABLE iam_user_role_bindings DROP INDEX idx_iam_binding_scope_type;
DROP TABLE IF EXISTS iam_scope_types;
