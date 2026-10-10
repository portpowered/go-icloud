package contracts_test

// Independent expected spellings keep malformed contract controls stable.
const (
	accountDevicesResponseSchema     = "AccountDevicesResponse"
	accountStorageResponseSchema     = "AccountStorageResponse"
	accountStorageSubscriptionPath   = "/acsegateway/v3/accounts/{dsid}/subscriptions/features/cloud.storage/plan-summary"
	clientModelsSchemaPath           = "../../api/client-models.openapi.yaml"
	authenticationCountryCodeField   = "accountCountryCode"
	unregisteredContractPath         = "/unregistered"
	findMyEraseDeviceSchema          = "FindMyEraseDevice"
	unknownContractPath              = "/unknown"
	photoSyncModelsPath              = "../../api/photo-sync-models.openapi.yaml"
	photoMaterializeModelsPath       = "../../api/photo-materialize.openapi.yaml"
	reminderCreationTokensSchema     = "ReminderCreationTokensMap"
	reminderUpdateTokensSchema       = "ReminderUpdateTokensMap"
	reminderDeletionResolutionSchema = "ReminderDeletionResolutionMap"
	reminderCreateHashtagFixture     = "create-hashtag-success"
	reminderCreateRecurrenceFixture  = "create-recurrence-rule-success"
	reminderAddLocationFixture       = "add-location-trigger-success"
	reminderCreateBasicFixture       = "create-basic"
	reminderRecordTypeName           = "Reminder"
	reminderCreationRequestSchema    = "ReminderCreationRequest"
	reminderUpdateRequestSchema      = "ReminderUpdateRequest"
	reminderDeletionRequestSchema    = "ReminderDeletionRequest"
	reminderNotesDocumentField       = "NotesDocument"
	remindersLookupOperation         = "RemindersLookupRecords"
	remindersQueryOperation          = "RemindersQueryRecords"
	remindersModifyOperation         = "RemindersModifyRecords"
	remindersChangesOperation        = "RemindersZoneChanges"
	remindersZonesOperation          = "RemindersListZones"
	reminderAssetOriginControl       = "asset-origin"
)

// Independent literals preserve schema spellings and negative controls.
const (
	//nolint:gosec // GO-15: this is the schema field spelling, not a credential.
	authWebTokenField               = "dsWebAuthToken"
	nullableErrorControl            = "nullable-error"
	contractSyntheticName           = "synthetic"
	issuerMethodControl             = "issuer-method"
	unissuedContractOrigin          = "https://unissued.example.invalid"
	findMyLostDeviceSchema          = "FindMyLostDevice"
	repeatedParameterControl        = "repeated"
	photoOriginalResourceKey        = "resOriginalRes"
	photoRecentInvalidJSONFixture   = "photos-recently-added-shared-invalid-json.json"
	reminderURLAttachmentFixture    = "create-url-attachment-success"
	reminderDeleteAttachmentFixture = "delete-attachment-success"
	reminderUpdateBasicFixture      = "update-basic"
	reminderDeleteFixture           = "delete-success"
	reminderRecordErrorOutcome      = "record-error"
	reminderFutureTokensField       = "futureTokens"
	reminderTitleDocumentField      = "TitleDocument"
	reminderAssetQueryControl       = "asset-query"
)

// Independent literals preserve synthetic controls and Source spellings.
const (
	contractExchangeFailureFormat           = "%s exchange %d: %v"
	contractAuthModelsPath                  = "../../api/external/auth-models.openapi.yaml"
	contractAuthAPIPath                     = "../../api/external/auth.openapi.yaml"
	contractFindMyFixturePattern            = "../replay/fixtures/synthetic/http/findmy-*.json"
	contractContentTypeField                = "Content-Type"
	contractExpectedFindMyInitializeRequest = "FindMyInitializeRequest"
	contractExpectedFindMyPlaySound         = "FindMyPlaySound"
	contractExpectedFindMySendMessage       = "FindMySendMessage"
	contractExpectedExtendedLogin           = "extended_login"
	contractExpectedIssuerPrefix            = "issuer-prefix"
	contractExpectedRequired                = "required"
	contractQueryControl                    = "token-query"
	contractTrustField                      = "trustToken"
)
