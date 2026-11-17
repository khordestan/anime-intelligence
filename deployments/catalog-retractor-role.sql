-- Retire reviewed unused public mappings only; password is provisioned separately.
DO $role$ BEGIN
 CREATE ROLE anime_catalog_retractor LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE
 NOINHERIT NOREPLICATION NOBYPASSRLS;
EXCEPTION WHEN duplicate_object THEN NULL;
END $role$;
GRANT USAGE ON SCHEMA public TO anime_catalog_retractor;
GRANT SELECT ON anime,anime_external_ids,data_sources,catalog_metadata,catalog_base_metadata,
 catalog_field_choices,catalog_field_choice_audits,catalog_schedule_observations,
 airing_schedule,anime_relations,catalog_fetch_jobs,source_records,catalog_identity_retractions TO anime_catalog_retractor;
GRANT UPDATE(display_name) ON data_sources TO anime_catalog_retractor;
GRANT DELETE ON anime_external_ids TO anime_catalog_retractor;
GRANT INSERT ON catalog_identity_retractions TO anime_catalog_retractor;
