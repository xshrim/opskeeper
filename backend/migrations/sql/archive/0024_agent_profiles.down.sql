DELETE FROM resources WHERE kind = 'AgentProfile';
DELETE FROM resource_schemas WHERE kind = 'AgentProfile' AND version = 1;
