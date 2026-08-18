DELETE FROM skill_versions AS version
 USING resources AS resource
 WHERE version.skill_resource_id = resource.id
   AND version.version = 2
   AND resource.kind = 'Skill'
   AND resource.config->>'owner' = 'OpsKeeper builtin';

UPDATE skill_versions AS version
   SET status = 'published', published_at = COALESCE(version.published_at, now())
  FROM resources AS resource
 WHERE version.skill_resource_id = resource.id
   AND version.version = 1
   AND resource.kind = 'Skill'
   AND resource.config->>'owner' = 'OpsKeeper builtin';
