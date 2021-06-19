package config

const EnvServerPort = "PORT"

const EnvDatabaseUrl = "DATABASE_URL"
const EnvReplicaDatabaseUrl = "REPLICA_DATABASE_URL"

const EnvMigrate = "DO_MIGRATION"
const EnvMigrationPath = "MIGRATION_PATH"

const EnvOauth2JwtVerifyAudience = "OAUTH2_JWT_VERIFY_AUDIENCE"
const EnvOauth2JwtVerifyIssuer = "OAUTH2_JWT_VERIFY_ISSUER"

const EnvStorageProvider = "STORAGE_PROVIDER"
const EnvStorageEncryptionPhrase = "ENCRYPTION_PHRASE"


const EnvFileAccessServerUrl = "FILE_ACCESS_SERVER_URL"

const EnvQueueFileSync = "QUEUE_FILE_SYNC"
const QueueFileSyncName = "file_model_sync"

const EnvQueueFileAuditSync = "QUEUE_FILE_AUDIT_SYNC"
const QueueFileAuditSyncName = "file_audit_model_sync"

const EnvCsrfSecret = "CSRF_SECRET"