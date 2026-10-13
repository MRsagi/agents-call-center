package cli

import "github.com/MRsagi/agents-call-center/internal/apiclient"

type (
	Client           = apiclient.Client
	CreateCallInput  = apiclient.CreateCallInput
	Health           = apiclient.Health
	QueueSummary     = apiclient.QueueSummary
	StatusSummary    = apiclient.StatusSummary
	VoicemailSummary = apiclient.VoicemailSummary
)

func BaseURL() string           { return apiclient.BaseURL() }
func NewClient(baseURL string) *Client { return apiclient.NewClient(baseURL) }
