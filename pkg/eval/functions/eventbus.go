package functions

import (
	"strings"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/gamedb"
)

// fnPublish implements publish(<queue_name>, <json_data>).
// Publishes an event to a named queue. Returns 1 on success, #-1 ERROR on failure.
func fnPublish(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 || ctx.GameState == nil {
		buf.WriteString("#-1 FUNCTION (PUBLISH) EXPECTS 2 ARGUMENTS")
		return
	}
	queueName := strings.TrimSpace(args[0])
	data := strings.TrimSpace(args[1])

	result := ctx.GameState.EventBusPublish(ctx.Player, queueName, data)
	if result != "" {
		buf.WriteString(result)
	} else {
		buf.WriteString("1")
	}
}

// fnSubscribe implements subscribe(<object>/<attr>, <queue_name>[, <bind>]).
// Subscribes an object's attribute to a queue.
func fnSubscribe(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 || ctx.GameState == nil {
		buf.WriteString("#-1 FUNCTION (SUBSCRIBE) EXPECTS 2-3 ARGUMENTS")
		return
	}

	// Parse object/attr
	objAttr := strings.TrimSpace(args[0])
	parts := strings.SplitN(objAttr, "/", 2)
	if len(parts) != 2 {
		buf.WriteString("#-1 INVALID OBJECT/ATTR")
		return
	}

	obj := resolveDBRef(ctx, parts[0])
	if obj == gamedb.Nothing {
		buf.WriteString("#-1 INVALID OBJECT")
		return
	}

	// Must control the target object to subscribe it
	if !ctx.GameState.Controls(ctx.Player, obj) {
		buf.WriteString("#-1 PERMISSION DENIED")
		return
	}

	attr := strings.ToUpper(strings.TrimSpace(parts[1]))
	queueName := strings.TrimSpace(args[1])

	// Optional bind argument
	bind := gamedb.Nothing
	if len(args) >= 3 {
		bindStr := strings.TrimSpace(args[2])
		if bindStr != "" {
			bind = resolveDBRef(ctx, bindStr)
			if bind == gamedb.Nothing {
				buf.WriteString("#-1 INVALID BIND TARGET")
				return
			}
		}
	}

	result := ctx.GameState.EventBusSubscribe(obj, attr, queueName, bind)
	if result != "" {
		buf.WriteString(result)
	} else {
		buf.WriteString("1")
	}
}

// fnUnsubscribe implements unsubscribe(<object>/<attr>, <queue_name>).
func fnUnsubscribe(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if len(args) < 2 || ctx.GameState == nil {
		buf.WriteString("#-1 FUNCTION (UNSUBSCRIBE) EXPECTS 2 ARGUMENTS")
		return
	}

	objAttr := strings.TrimSpace(args[0])
	parts := strings.SplitN(objAttr, "/", 2)
	if len(parts) != 2 {
		buf.WriteString("#-1 INVALID OBJECT/ATTR")
		return
	}

	obj := resolveDBRef(ctx, parts[0])
	if obj == gamedb.Nothing {
		buf.WriteString("#-1 INVALID OBJECT")
		return
	}

	// Must control the target object to unsubscribe it
	if !ctx.GameState.Controls(ctx.Player, obj) {
		buf.WriteString("#-1 PERMISSION DENIED")
		return
	}

	attr := strings.ToUpper(strings.TrimSpace(parts[1]))
	queueName := strings.TrimSpace(args[1])

	result := ctx.GameState.EventBusUnsubscribe(obj, attr, queueName)
	if result != "" {
		buf.WriteString(result)
	} else {
		buf.WriteString("1")
	}
}

// fnQueues implements queues([<name>[, <mode>]]).
// No args: list all queue names. With name: return info JSON.
// With name + "subs": list subscriber dbrefs. With name + "stats": return stats JSON.
func fnQueues(ctx *eval.EvalContext, args []string, buf *strings.Builder, _, _ gamedb.DBRef) {
	if ctx.GameState == nil {
		buf.WriteString("#-1 NOT AVAILABLE")
		return
	}

	if len(args) == 0 {
		buf.WriteString(ctx.GameState.EventBusQueueList())
		return
	}

	queueName := strings.TrimSpace(args[0])
	mode := ""
	if len(args) >= 2 {
		mode = strings.ToLower(strings.TrimSpace(args[1]))
	}

	result := ctx.GameState.EventBusQueueInfo(queueName, mode)
	buf.WriteString(result)
}
