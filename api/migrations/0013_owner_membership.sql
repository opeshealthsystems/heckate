-- The API connects as the schema owner and then drops to hekate_app (SET ROLE) so row-level security
-- applies. A superuser can always do that; an ordinary owner needs membership with the right to SET
-- the role. PostgreSQL 16 stopped granting that to a role's creator automatically.
DO $$
BEGIN
  BEGIN
    EXECUTE 'GRANT hekate_app TO CURRENT_USER WITH SET TRUE';   -- PostgreSQL 16 and later
  EXCEPTION WHEN syntax_error THEN
    EXECUTE 'GRANT hekate_app TO CURRENT_USER';                   -- earlier versions
  END;
EXCEPTION WHEN duplicate_object OR invalid_grant_operation THEN
  NULL; -- already a member (for example a superuser)
END $$;

-- SECURITY DEFINER functions run as the table owner. With FORCE ROW LEVEL SECURITY the owner is bound by
-- the policies too, and an ordinary (non-superuser) owner would see no rows: the website enquiry insert
-- would be refused and public document verification would find nothing. The two tables below are only
-- reached through such functions, so the owner is exempted. The application role (hekate_app) is not the
-- owner and stays fully subject to row-level security.
ALTER TABLE crm.leads NO FORCE ROW LEVEL SECURITY;
ALTER TABLE documents.register NO FORCE ROW LEVEL SECURITY;
