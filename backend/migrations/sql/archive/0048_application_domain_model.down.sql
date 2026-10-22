DROP TRIGGER IF EXISTS application_dependencies_validate_target ON application_dependencies;
DROP TRIGGER IF EXISTS application_instances_validate_target ON application_instances;
DROP FUNCTION IF EXISTS validate_application_target();
DROP TABLE IF EXISTS application_dependencies;
DROP TABLE IF EXISTS application_instances;
DROP TABLE IF EXISTS applications;
