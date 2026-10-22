UPDATE resource_schemas SET status = 'active' WHERE kind = 'Application' AND version = 1;
DELETE FROM resource_schemas WHERE kind = 'Application' AND version = 2;
