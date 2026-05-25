package helps

import (
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// PreserveReasoningContent ensures assistant messages with tool_calls retain
// reasoning_content in the request body. Providers like MiMo and DeepSeek
// require reasoning_content to be passed back verbatim in multi-turn tool-call
// scenarios. Injecting fabricated content (placeholders or content text) would
// corrupt the model context per MiMo's documented requirement.
//
// Some client SDKs (e.g. @ai-sdk/openai-compatible) strip reasoning_content
// from conversation history before sending to CPA. Since req.Payload and
// opts.OriginalRequest are the same rawJSON, comparing them is a no-op.
// Instead, this function scans the conversation history to inherit the latest
// non-empty reasoning_content into assistant messages that are missing it.
//
// Behavior:
//   - Tracks the latest non-empty reasoning_content from prior assistant messages
//   - Empty reasoning_content ("") does NOT overwrite a prior non-empty value
//   - JSON null reasoning_content is treated as "seen but empty" (hasSeenReasoning
//     is set, but latestReasoning is not updated)
//   - Injects the inherited reasoning into assistant messages with tool_calls
//   - If latestReasoning is empty (all prior were empty or null), injects empty string
//   - If no prior message ever had reasoning_content, returns body unchanged
//   - When client SDK already preserves reasoning_content, this function is a no-op
//     (only iterates messages for existence checks, no sjson modifications)
//
// Does NOT fabricate content — no fallback to content text or placeholders.
func PreserveReasoningContent(body []byte) []byte {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body
	}

	msgs := gjson.GetBytes(body, "messages")
	if !msgs.Exists() || !msgs.IsArray() {
		return body
	}

	arr := msgs.Array()
	out := body

	latestReasoning := ""
	hasSeenReasoning := false

	for i, msg := range arr {
		role := msg.Get("role").String()
		if role != "assistant" {
			continue
		}

		rc := msg.Get("reasoning_content")
		if rc.Exists() {
			// rc.Exists() is true for both explicit values and JSON null.
			// rc.String() returns "" for null, which TrimSpace rejects,
			// so null does not overwrite a prior non-empty latestReasoning.
			// hasSeenReasoning is still set to true, enabling injection for
			// subsequent assistant messages that lack reasoning_content.
			if rcText := strings.TrimSpace(rc.String()); rcText != "" {
				latestReasoning = rcText
			}
			hasSeenReasoning = true
			continue
		}

		if !hasSeenReasoning {
			continue
		}

		toolCalls := msg.Get("tool_calls")
		if !toolCalls.Exists() || !toolCalls.IsArray() || len(toolCalls.Array()) == 0 {
			continue
		}

		path := "messages." + strconv.Itoa(i) + ".reasoning_content"
		updated, err := sjson.SetBytes(out, path, latestReasoning)
		if err == nil {
			out = updated
		} else {
			log.Debugf("preserve reasoning_content: sjson.SetBytes failed at %s: %v", path, err)
		}
	}

	return out
}
