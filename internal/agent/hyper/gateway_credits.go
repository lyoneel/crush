package hyper

import "strings"

// HyperModelIDPrefix marks the model ids a gateway uses when it serves
// Hyper models: a gateway lists them as "Hyper/<model>". Matching is
// case-insensitive.
const HyperModelIDPrefix = "Hyper/"

// IsGatewayHyperModel reports whether a model id names a Hyper model
// served through an OpenAI-compatible gateway: the Hyper/ prefix, or a
// /hyper/ path segment anywhere in the id, in any letter case. Such
// models consume the same hypercredit balance as the hyper provider.
func IsGatewayHyperModel(_, modelID string) bool {
	id := strings.ToLower(modelID)
	prefix := strings.ToLower(HyperModelIDPrefix)
	return strings.HasPrefix(id, prefix) || strings.Contains(id, "/"+prefix)
}

func init() {
	RegisterCreditsMatcher(IsGatewayHyperModel)
}
