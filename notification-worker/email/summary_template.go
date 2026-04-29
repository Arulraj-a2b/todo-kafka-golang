package email

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"

	"notification-worker/models"
)

type summaryViewItem struct {
	Title  string
	Kind   string // "overdue" | "due_today" | "other"
	Detail string // e.g. "due in 2h" or "overdue by 3 days"
	Bg     string
	Fg     string
}

type summaryView struct {
	UserEmail   string
	Date        string
	GreetingHr  string
	Counts      models.DailySummaryCounts
	HasOverdue  bool
	HasDueToday bool
	Highlights  []summaryViewItem
	TotalActive int
}

var summaryTmpl = template.Must(template.New("summary").Parse(`<!DOCTYPE html>
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
            <div style="font-size:13px; font-weight:700; color:#0e7490; letter-spacing:0.08em; text-transform:uppercase;">📋 DAILY SUMMARY</div>
            <h1 style="margin:6px 0 0 0; font-size:22px; font-weight:700; color:#111827; line-height:1.25;">Good {{.GreetingHr}}, {{.UserEmail}}</h1>
            <div style="margin-top:4px; font-size:14px; color:#6b7280;">{{.Date}}</div>
          </td>
        </tr>

        <tr>
          <td style="padding:20px 32px 8px 32px;">
            <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td align="center" style="padding:14px; background:#f9fafb; border-radius:8px; border:1px solid #e5e7eb;">
                  <div style="font-size:11px; color:#6b7280; text-transform:uppercase; letter-spacing:0.05em; font-weight:700;">Pending</div>
                  <div style="font-size:24px; font-weight:700; color:#1e40af; margin-top:4px;">{{.Counts.Pending}}</div>
                </td>
                <td width="8"></td>
                <td align="center" style="padding:14px; background:#fef3c7; border-radius:8px; border:1px solid #fde68a;">
                  <div style="font-size:11px; color:#92400e; text-transform:uppercase; letter-spacing:0.05em; font-weight:700;">In Progress</div>
                  <div style="font-size:24px; font-weight:700; color:#92400e; margin-top:4px;">{{.Counts.InProgress}}</div>
                </td>
                <td width="8"></td>
                <td align="center" style="padding:14px; background:#fee2e2; border-radius:8px; border:1px solid #fecaca;">
                  <div style="font-size:11px; color:#991b1b; text-transform:uppercase; letter-spacing:0.05em; font-weight:700;">Overdue</div>
                  <div style="font-size:24px; font-weight:700; color:#991b1b; margin-top:4px;">{{.Counts.Overdue}}</div>
                </td>
              </tr>
              <tr><td colspan="5" height="8"></td></tr>
              <tr>
                <td align="center" style="padding:14px; background:#dbeafe; border-radius:8px; border:1px solid #bfdbfe;">
                  <div style="font-size:11px; color:#1e40af; text-transform:uppercase; letter-spacing:0.05em; font-weight:700;">Due Today</div>
                  <div style="font-size:24px; font-weight:700; color:#1e40af; margin-top:4px;">{{.Counts.DueToday}}</div>
                </td>
                <td width="8"></td>
                <td align="center" style="padding:14px; background:#d1fae5; border-radius:8px; border:1px solid #a7f3d0;">
                  <div style="font-size:11px; color:#065f46; text-transform:uppercase; letter-spacing:0.05em; font-weight:700;">Done Yesterday ✅</div>
                  <div style="font-size:24px; font-weight:700; color:#065f46; margin-top:4px;">{{.Counts.CompletedYesterday}}</div>
                </td>
                <td width="8"></td>
                <td></td>
              </tr>
            </table>
          </td>
        </tr>

        {{if .Highlights}}
        <tr>
          <td style="padding:20px 32px 0 32px;">
            <div style="font-size:13px; color:#6b7280; text-transform:uppercase; letter-spacing:0.05em; font-weight:700; margin-bottom:8px;">Top items</div>
            <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0">
              {{range .Highlights}}
              <tr>
                <td style="padding:10px 12px; border-radius:6px; background:{{.Bg}}; color:{{.Fg}};">
                  <div style="font-size:14px; font-weight:600;">{{.Title}}</div>
                  <div style="font-size:12px; opacity:0.9; margin-top:2px;">{{.Detail}}</div>
                </td>
              </tr>
              <tr><td height="6"></td></tr>
              {{end}}
            </table>
          </td>
        </tr>
        {{end}}

        <tr>
          <td style="padding:24px 32px 28px 32px;">
            <div style="border-top:1px solid #e5e7eb; padding-top:18px; font-size:12px; color:#9ca3af; text-align:center; line-height:1.6;">
              You have <strong>{{.TotalActive}}</strong> active todos.<br>
              Open Todo App to manage them.
            </div>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>
</body>
</html>`))

func greetingForHour(h int) string {
	switch {
	case h < 12:
		return "morning"
	case h < 17:
		return "afternoon"
	default:
		return "evening"
	}
}

func RenderDailySummary(data models.DailySummaryData) (subject, text, html string) {
	c := data.Counts
	totalActive := c.Pending + c.InProgress
	subject = fmt.Sprintf("📋 Your daily summary — %d todos waiting", totalActive)

	highlights := make([]summaryViewItem, 0, len(data.Highlights))
	for _, h := range data.Highlights {
		var item summaryViewItem
		item.Title = h.Title
		item.Kind = h.Kind
		switch h.Kind {
		case "overdue":
			item.Bg = "#fee2e2"
			item.Fg = "#991b1b"
			if h.DueDate != nil {
				item.Detail = "overdue · was due " + h.DueDate.Format("Mon Jan 2, 15:04")
			} else {
				item.Detail = "overdue"
			}
		case "due_today":
			item.Bg = "#dbeafe"
			item.Fg = "#1e40af"
			if h.DueDate != nil {
				item.Detail = "due today at " + h.DueDate.Format("15:04")
			} else {
				item.Detail = "due today"
			}
		default:
			item.Bg = "#f3f4f6"
			item.Fg = "#374151"
			if h.DueDate != nil {
				item.Detail = "due " + h.DueDate.Format("Mon Jan 2, 15:04")
			}
		}
		highlights = append(highlights, item)
	}

	v := summaryView{
		UserEmail:   data.UserEmail,
		Date:        data.Date,
		GreetingHr:  greetingForHour(9), // we send at 9 AM by default
		Counts:      c,
		HasOverdue:  c.Overdue > 0,
		HasDueToday: c.DueToday > 0,
		Highlights:  highlights,
		TotalActive: totalActive,
	}

	var buf bytes.Buffer
	_ = summaryTmpl.Execute(&buf, v)
	html = buf.String()

	var t strings.Builder
	t.WriteString(fmt.Sprintf("Daily summary — %s\n\n", data.Date))
	t.WriteString(fmt.Sprintf("Pending:             %d\n", c.Pending))
	t.WriteString(fmt.Sprintf("In progress:         %d\n", c.InProgress))
	t.WriteString(fmt.Sprintf("Due today:           %d\n", c.DueToday))
	t.WriteString(fmt.Sprintf("Overdue:             %d\n", c.Overdue))
	t.WriteString(fmt.Sprintf("Completed yesterday: %d\n\n", c.CompletedYesterday))
	if len(data.Highlights) > 0 {
		t.WriteString("Top items:\n")
		for _, h := range data.Highlights {
			t.WriteString("  - " + h.Title + " (" + h.Kind + ")\n")
		}
	}
	text = t.String()
	return
}
