UPDATE role_permissions
   SET permission = 'ai_engine:default_manage'
 WHERE permission = 'engine:manage';
