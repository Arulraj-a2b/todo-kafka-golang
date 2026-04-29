package email

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"time"

	"notification-worker/models"
)

type overdueData struct {
	Title       string
	Priority    string
	PriorityBg  string
	PriorityFg  string
	OverdueBy   string
	DueDate     string
	Tags        []string
	AccentColor string
}

var overdueTmpl = template.Must(template.New("overdue").Parse(`<!DOCTYPE html>
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
            <div style="font-size:13px; font-weight:700; color:{{.AccentColor}}; letter-spacing:0.08em; text-transform:uppercase;">⏰ TODO OVERDUE</div>
            <h1 style="margin:6px 0 0 0; font-size:22px; font-weight:700; color:#111827; line-height:1.25;">A todo just went overdue</h1>
          </td>
        </tr>
        <tr>
          <td style="padding:20px 32px 8px 32px;">
            <div style="background:#fff7ed; border:1px solid #fed7aa; border-left:4px solid {{.AccentColor}}; border-radius:8px; padding:16px 18px;">
              <div style="font-size:11px; color:#9a3412; text-transform:uppercase; letter-spacing:0.05em; font-weight:700;">Title</div>
              <div style="margin-top:6px; font-size:18px; font-weight:600; color:#111827; line-height:1.4; word-break:break-word;">{{.Title}}</div>
            </div>
          </td>
        </tr>
        <tr>
          <td style="padding:8px 32px 8px 32px;">
            <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin-top:8px;">
              <tr>
                <td style="padding:12px 0; border-bottom:1px solid #f1f3f5; font-size:13px; color:#6b7280; width:120px;">Overdue by</td>
                <td style="padding:12px 0; border-bottom:1px solid #f1f3f5; font-size:14px; font-weight:600; color:#9a3412;">{{.OverdueBy}}</td>
              </tr>
              <tr>
                <td style="padding:12px 0; border-bottom:1px solid #f1f3f5; font-size:13px; color:#6b7280;">Was due</td>
                <td style="padding:12px 0; border-bottom:1px solid #f1f3f5; font-size:14px; color:#111827;">{{.DueDate}}</td>
              </tr>
              <tr>
                <td style="padding:12px 0; border-bottom:1px solid #f1f3f5; font-size:13px; color:#6b7280;">Priority</td>
                <td style="padding:12px 0; border-bottom:1px solid #f1f3f5;">
                  <span style="display:inline-block; padding:4px 12px; border-radius:9999px; background:{{.PriorityBg}}; color:{{.PriorityFg}}; font-size:12px; font-weight:700; text-transform:capitalize;">{{.Priority}}</span>
                </td>
              </tr>
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
              You're receiving this because a todo passed its due date.<br>
              Open Todo App to mark it complete or reschedule.
            </div>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>
</body>
</html>`))

// formatOverdueBy turns a duration into a friendly "X minutes ago" string.
func formatOverdueBy(seconds int64) string {
	d := time.Duration(seconds) * time.Second
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d seconds", seconds)
	case d < time.Hour:
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d days", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%d days", int(d.Hours()/24))
	}
}

func RenderOverdue(data models.OverdueData) (subject, text, html string) {
	subject = "⏰ Overdue: " + data.Todo.Title

	priorityBg, priorityFg := priorityColors(data.Todo.Priority)
	dueStr := ""
	if data.Todo.DueDate != nil {
		dueStr = data.Todo.DueDate.Format("Mon, 02 Jan 2006 — 15:04 MST")
	}
	overdueBy := formatOverdueBy(data.OverdueBySeconds)

	d := overdueData{
		Title:       data.Todo.Title,
		Priority:    string(data.Todo.Priority),
		PriorityBg:  priorityBg,
		PriorityFg:  priorityFg,
		OverdueBy:   overdueBy,
		DueDate:     dueStr,
		Tags:        data.Todo.Tags,
		AccentColor: "#ea580c", // orange-600
	}
	var buf bytes.Buffer
	_ = overdueTmpl.Execute(&buf, d)
	html = buf.String()

	var t strings.Builder
	t.WriteString("Your todo is overdue.\n\n")
	t.WriteString("Title:    " + data.Todo.Title + "\n")
	t.WriteString("Overdue:  " + overdueBy + "\n")
	if dueStr != "" {
		t.WriteString("Was due:  " + dueStr + "\n")
	}
	t.WriteString("Priority: " + string(data.Todo.Priority) + "\n")
	if len(data.Todo.Tags) > 0 {
		t.WriteString("Tags:     " + strings.Join(data.Todo.Tags, ", ") + "\n")
	}
	text = t.String()
	return
}
