package zscaler

import (
	"reflect"

	"github.com/dvmrry/zscalerctl/internal/resources"

	zccfailopen "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcc/services/failopen_policy"
)

func reviewedSDKShapesZCC() []sdkShapeReview {
	return []sdkShapeReview{
		{
			name:         "zccfailopen.WebFailOpenPolicy",
			resource:     resources.ProductZCC,
			resourceName: resourceZCCFailOpenPolicy,
			typ:          reflect.TypeOf(zccfailopen.WebFailOpenPolicy{}),
			catalogFields: []string{
				"id",
				"active",
				"enableFailOpen",
				"enableCaptivePortalDetection",
				"captivePortalWebSecDisableMinutes",
				"enableStrictEnforcementPrompt",
				"strictEnforcementPromptDelayMinutes",
				"strictEnforcementPromptMessage",
				"enableWebSecOnProxyUnreachable",
				"enableWebSecOnTunnelFailure",
				"tunnelFailureRetryCount",
				"createdBy",
				"editedBy",
				"companyId",
			},
		},
	}
}
