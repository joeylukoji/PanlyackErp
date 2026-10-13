package ports

import "panlyackerp/internal/modules/accounting/domain"

type Posting struct {
	OrganizationID, SourceID, Reference, Date, Currency string
	Entry                                               domain.Entry
}

type JournalRow struct{ Reference, Date, SourceID, Code, Name, Debit, Credit string }
