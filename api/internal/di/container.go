package di

import (
	commonobservability "github.com/huynhanx03/go-common/pkg/common/observability"

	debuglevel "github.com/huynhanx03/judgify/internal/application/workflow/debuglevel"
	judgeOperations "github.com/huynhanx03/judgify/internal/judge/operations"
	audithttp "github.com/huynhanx03/judgify/internal/platform/audit/adapters/driver/http"
	"github.com/huynhanx03/judgify/internal/platform/operation"
	operationhttp "github.com/huynhanx03/judgify/internal/platform/operation/adapters/driver/http"
	realtimeDi "github.com/huynhanx03/judgify/internal/realtime/di"
)

// Container holds all domain-specific dependency containers.
type Container struct {
	Audit                *audithttp.Handler
	Identity             *IdentityContainer
	Judge                *judgeOperations.HTTPHandler
	Problem              *ProblemContainer
	Cultivation          *CultivationContainer
	Submission           *SubmissionContainer
	Contest              *ContestContainer
	Material             *MaterialContainer
	Notification         *NotificationContainer
	NotificationCampaign *CampaignContainer
	Operation            *operation.Components
	OperationHTTP        *operationhttp.Handler
	OperationRunner      *operation.Runner
	Realtime             *realtimeDi.RealtimeContainer
	DebugLevel           *debuglevel.HTTPHandler
	DebugWindow          *commonobservability.DebugWindow
}
