-- DESTRUCTIVE, OFFLINE RECOVERY ONLY. Run against an isolated restored database
-- as its recovery administrator, with all API/worker instances stopped.
-- This blocks password login until a fresh reset, revokes sessions/proofs and
-- discards ALL restored identity requests/mail. No queue keys are needed.
-- Reconcile post-backup deletions/account changes before permitting recovery.
-- See docs/BACKUP_RECOVERY.md. This is deliberately not a migration.
BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';
SET LOCAL search_path = pg_catalog, public;
DO $guard$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles
                 WHERE rolname = current_user AND (rolsuper OR rolbypassrls)) THEN
    RAISE EXCEPTION 'offline recovery requires an RLS-bypassing administrator';
  END IF;
END $guard$;

LOCK TABLE public.users, public.auth_sessions, public.auth_challenges,
  public.identity_requests, public.identity_outbox IN ACCESS EXCLUSIVE MODE;

-- Password reset clears locked_until; disabled accounts remain disabled.
UPDATE public.users SET locked_until = 'infinity', updated_at = now()
 WHERE disabled_at IS NULL;
UPDATE public.auth_sessions SET revoked_at = now() WHERE revoked_at IS NULL;
UPDATE public.auth_challenges SET revoked_at = now()
 WHERE consumed_at IS NULL AND revoked_at IS NULL;
DELETE FROM public.identity_requests;
DELETE FROM public.identity_outbox;
COMMIT;
