package email

import (
	"bytes"
	"html/template"
	"strings"
	"time"

	"todo-service/models"
)

type todoEmailData struct {
	HeaderLabel string
	Heading     string
	Verb        string
	AccentColor string
	Title       string
	StatusLabel string
	StatusBg    string
	StatusFg    string
	Priority    string
	PriorityBg  string
	PriorityFg  string
	DueDate     string
	Tags        []string
	Timestamp   string
}

func statusColors(s models.Status) (bg, fg, label string) {
	label = strings.ReplaceAll(string(s), "_", " ")
	switch s {
	case models.StatusCompleted:
		return "#d1fae5", "#065f46", label
	case models.StatusInProgress:
		return "#fef3c7", "#92400e", label
	case models.StatusDeleted:
		return "#fee2e2", "#991b1b", label
	default: // pending
		return "#dbeafe", "#1e40af", label
	}
}

func priorityColors(p models.Priority) (bg, fg string) {
	switch p {
	case models.PriorityHigh:
		return "#fee2e2", "#991b1b"
	case models.PriorityMedium:
		return "#fef3c7", "#92400e"
	default: // low
		return "#dbeafe", "#1e40af"
	}
}

func formatDue(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("Mon, 02 Jan 2006 — 15:04 MST")
}

// htmlTmpl uses table-based layout + inline styles for broad email-client compatibility
// (Gmail strips <style> blocks, Outlook ignores flexbox/grid). Max-width 600px is the
// de-facto email standard.
var htmlTmpl = template.Must(template.New("todo-email").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
</head>
<body style="margin:0; padding:0; background:#f4f6f8; font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif; color:#111827;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background:#f4f6f8; padding:32px 16px;">
  <tr>
    <td align="center">
      <table role="presentation" width="600" cellpadding="0" cellspacing="0" border="0" style="max-width:600px; width:100%; background:#ffffff; border-radius:12px; overflow:hidden; box-shadow:0 1px 3px rgba(0,0,0,0.05);">
        <tr>
          <td style="padding:28px 32px 8px 32px;">
            <div style="font-size:13px; font-weight:700; color:{{.AccentColor}}; letter-spacing:0.08em; text-transform:uppercase;">{{.HeaderLabel}}</div>
            <h1 style="margin:6px 0 0 0; font-size:22px; font-weight:700; color:#111827; line-height:1.25;">{{.Heading}}</h1>
          </td>
        </tr>
        <tr>
          <td style="padding:20px 32px 8px 32px;">
            <div style="background:#f9fafb; border:1px solid #e5e7eb; border-left:4px solid {{.AccentColor}}; border-radius:8px; padding:16px 18px;">
              <div style="font-size:11px; color:#6b7280; text-transform:uppercase; letter-spacing:0.05em; font-weight:700;">Title</div>
              <div style="margin-top:6px; font-size:18px; font-weight:600; color:#111827; line-height:1.4; word-break:break-word;">{{.Title}}</div>
            </div>
          </td>
        </tr>
        <tr>
          <td style="padding:8px 32px 8px 32px;">
            <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin-top:8px;">
              <tr>
                <td style="padding:12px 0; border-bottom:1px solid #f1f3f5; font-size:13px; color:#6b7280; width:120px;">Status</td>
                <td style="padding:12px 0; border-bottom:1px solid #f1f3f5;">
                  <span style="display:inline-block; padding:4px 12px; border-radius:9999px; background:{{.StatusBg}}; color:{{.StatusFg}}; font-size:12px; font-weight:700; text-transform:capitalize;">{{.StatusLabel}}</span>
                </td>
              </tr>
              <tr>
                <td style="padding:12px 0; border-bottom:1px solid #f1f3f5; font-size:13px; color:#6b7280;">Priority</td>
                <td style="padding:12px 0; border-bottom:1px solid #f1f3f5;">
                  <span style="display:inline-block; padding:4px 12px; border-radius:9999px; background:{{.PriorityBg}}; color:{{.PriorityFg}}; font-size:12px; font-weight:700; text-transform:capitalize;">{{.Priority}}</span>
                </td>
              </tr>
              {{if .DueDate}}
              <tr>
                <td style="padding:12px 0; border-bottom:1px solid #f1f3f5; font-size:13px; color:#6b7280;">Due date</td>
                <td style="padding:12px 0; border-bottom:1px solid #f1f3f5; font-size:14px; color:#111827;">{{.DueDate}}</td>
              </tr>
              {{end}}
              {{if .Tags}}
              <tr>
                <td style="padding:12px 0; font-size:13px; color:#6b7280; vertical-align:top;">Tags</td>
                <td style="padding:8px 0;">
                  {{range .Tags}}<span style="display:inline-block; margin:0 6px 6px 0; padding:4px 10px; border-radius:9999px; background:#eef2ff; color:#4338ca; font-size:12px; font-weight:600;">{{.}}</span>{{end}}
                </td>
              </tr>
              {{end}}
            </table>
          </td>
        </tr>
        <tr>
          <td style="padding:24px 32px 28px 32px;">
            <div style="border-top:1px solid #e5e7eb; padding-top:18px; font-size:12px; color:#9ca3af; text-align:center; line-height:1.6;">
              You're receiving this because you {{.Verb}}d a todo in Todo App.<br>
              <span style="color:#6b7280;">{{.Timestamp}}</span>
            </div>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>
</body>
</html>`))

func RenderTodo(action string, todo models.Todo) (subject, text, html string) {
	verb := "create"
	headerLabel := "TODO CREATED"
	heading := "Your todo was created"
	accent := "#4f46e5" // indigo
	subject = "Todo created: " + todo.Title
	if action == models.ActionUpdate {
		verb = "update"
		headerLabel = "TODO UPDATED"
		heading = "Your todo was updated"
		accent = "#0891b2" // cyan
		subject = "Todo updated: " + todo.Title
	}

	statusBg, statusFg, statusLabel := statusColors(todo.Status)
	priorityBg, priorityFg := priorityColors(todo.Priority)

	data := todoEmailData{
		HeaderLabel: headerLabel,
		Heading:     heading,
		Verb:        verb,
		AccentColor: accent,
		Title:       todo.Title,
		StatusLabel: statusLabel,
		StatusBg:    statusBg,
		StatusFg:    statusFg,
		Priority:    string(todo.Priority),
		PriorityBg:  priorityBg,
		PriorityFg:  priorityFg,
		DueDate:     formatDue(todo.DueDate),
		Tags:        todo.Tags,
		Timestamp:   todo.UpdatedAt.Format("Mon, 02 Jan 2006 — 15:04 MST"),
	}

	var buf bytes.Buffer
	_ = htmlTmpl.Execute(&buf, data)
	html = buf.String()

	var t strings.Builder
	t.WriteString(heading)
	t.WriteString("\n\n")
	t.WriteString("Title:    " + todo.Title + "\n")
	t.WriteString("Status:   " + statusLabel + "\n")
	t.WriteString("Priority: " + string(todo.Priority) + "\n")
	if todo.DueDate != nil {
		t.WriteString("Due:      " + formatDue(todo.DueDate) + "\n")
	}
	if len(todo.Tags) > 0 {
		t.WriteString("Tags:     " + strings.Join(todo.Tags, ", ") + "\n")
	}
	t.WriteString("\n" + data.Timestamp + "\n")
	text = t.String()

	return
}
