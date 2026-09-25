package model

type ActivityStatus uint8

const (
	ActivityDraft ActivityStatus = iota
	ActivityPendingReview
	ActivityRejected
	ActivityPublished
	ActivityCancelled
	ActivityFinished
)

type RegistrationStatus uint8

const (
	RegistrationPending RegistrationStatus = iota
	RegistrationApproved
	RegistrationRejected
	RegistrationCancelled
	RegistrationExpired
)

type TicketStatus uint8

const (
	TicketValid TicketStatus = iota
	TicketUsed
	TicketVoided
	TicketExpired
)

type StudentVerificationStatus uint8

const (
	VerificationInitialized StudentVerificationStatus = iota
	VerificationOCRProcessing
	VerificationPendingConfirm
	VerificationManualReview
	VerificationApproved
	VerificationRejected
	VerificationExpired
	VerificationCancelled
	VerificationOCRFailed
)
