UPDATE role_permissions
   SET permission = 'engine:manage'
 WHERE permission = 'ai_engine:default_manage';
