package screen

import (
	"strconv"

	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
)

// AccountPicker is the dialog a opens on a Claude cell: every account in the
// config's order, with its usage and whether it's at its limit, and the one
// the conversation runs on marked. Its keyboard is the form's: ↑↓ pick, ↵
// moves the conversation there, esc leaves. The accounts are read from the
// state on every frame, so the numbers stay live while it's open.
type AccountPicker struct {
	Cell     string
	Selected int
}

// NewAccountPicker opens the picker on the account the cell runs on, or says
// why there's nothing to pick: a single account, or no Claude up to move.
func NewAccountPicker(item protocol.Cell, accounts []protocol.Account) (*AccountPicker, string) {
	switch {
	case len(accounts) < 2:
		return nil, "Only one Claude account is set up. ts account add sets up another."
	case item.Account == "" && len(item.Tabs) > 0:
		return nil, item.Name + "'s claude tab isn't up. tab switches to it and brings it up."
	case item.Account == "":
		return nil, item.Name + " has no Claude running. r brings it back up."
	}
	picker := &AccountPicker{Cell: item.ID}
	for i, account := range accounts {
		if account.Name == item.Account {
			picker.Selected = i
		}
	}
	return picker, ""
}

// Key handles a key for the picker. It returns the move to send when a free
// account is chosen, the reason when the one chosen is at its limit — the
// picker stays open for another pick — and whether it stays open. Choosing
// the account the conversation is already on just closes it.
func (p *AccountPicker) Key(action keyboard.Action, accounts []protocol.Account, current string) (move *protocol.Move, refusal string, open bool) {
	p.Selected = min(max(p.Selected, 0), max(len(accounts)-1, 0))
	switch action {
	case keyboard.Cancel:
		return nil, "", false
	case keyboard.PreviousField:
		p.Selected = max(p.Selected-1, 0)
	case keyboard.NextField:
		p.Selected = min(p.Selected+1, max(len(accounts)-1, 0))
	case keyboard.Confirm:
		if len(accounts) == 0 {
			return nil, "", false
		}
		chosen := accounts[p.Selected]
		switch {
		case chosen.Name == current:
			return nil, "", false
		case chosen.Until != "":
			return nil, chosen.Name + " is at its limit until " + chosen.Until + ". ↑↓ picks another account.", true
		}
		return &protocol.Move{Cell: p.Cell, Account: chosen.Name}, "", false
	}
	return nil, "", true
}

// Draw builds the picker's box for the cell as it is now.
func (p *AccountPicker) Draw(width int, accounts []protocol.Account, item protocol.Cell) []string {
	inner := min(width-8, 60)
	var body []string
	for i, account := range accounts {
		usage := "–"
		if account.Quota != nil {
			usage = strconv.Itoa(account.Quota.Percent) + "%"
		}
		status, paintStatus := "", mutedColor
		if account.Until != "" {
			status, paintStatus = "at its limit until "+account.Until+"  ", warnColor
		}
		if account.Name == item.Account {
			status += "← current"
		}
		if i == p.Selected {
			row := pad(account.Name, 12) + pad(usage, 6) + status
			body = append(body, selectedBar+selectedRowColor.Render(" "+pad(row, max(inner-5, 1))))
			continue
		}
		body = append(body, "  "+barColor.Render(pad(account.Name, 12))+mutedColor.Render(pad(usage, 6))+paintStatus.Render(status))
	}
	body = append(body, "")
	// Moving restarts the conversation's Claude: whatever it's doing stops.
	if item.State == "working" || item.State == "approve" {
		body = append(body, "  "+warnColor.Render("moving cuts the turn in progress"))
	}
	body = append(body, "  "+hints("↑↓ choose", "↵ move here", "esc cancel"))
	return floatingBox("ACCOUNT · "+item.Name, body, inner)
}
