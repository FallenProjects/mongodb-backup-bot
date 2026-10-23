package main

import (
	"fmt"
	"strconv"

	"github.com/AshokShau/gotdbot"
)

func button(text, data string, style gotdbot.ButtonStyle) gotdbot.InlineKeyboardButton {
	if style == nil {
		style = &gotdbot.ButtonStyleDefault{}
	}
	return gotdbot.InlineKeyboardButton{Text: text, Style: style, Type: &gotdbot.InlineKeyboardButtonTypeCallback{Data: []byte(data)}}
}

func homeKeyboard() *gotdbot.ReplyMarkupInlineKeyboard {
	return &gotdbot.ReplyMarkupInlineKeyboard{Rows: [][]gotdbot.InlineKeyboardButton{
		{button("Help", "help", &gotdbot.ButtonStylePrimary{}), button("Privacy", "privacy", nil)},
		{{Text: "Source code", Style: &gotdbot.ButtonStyleLink{}, Type: &gotdbot.InlineKeyboardButtonTypeUrl{Url: "https://github.com/AshokShau/mongodb-backup-bot"}}},
	}}
}

func menuKeyboard() *gotdbot.ReplyMarkupInlineKeyboard {
	return &gotdbot.ReplyMarkupInlineKeyboard{Rows: [][]gotdbot.InlineKeyboardButton{
		{button("Back up all", "mongo:all", &gotdbot.ButtonStyleSuccess{}), button("Select database", "mongo:page:0", &gotdbot.ButtonStylePrimary{})},
		{button("Wipe databases", "mongo:wipe", &gotdbot.ButtonStyleDanger{})},
		{button("Cancel", "mongo:cancel", nil)},
	}}
}

func confirmKeyboard(action string) *gotdbot.ReplyMarkupInlineKeyboard {
	text, style := "Delete all user databases", gotdbot.ButtonStyle(&gotdbot.ButtonStyleDanger{})
	if action == "restore" {
		text, style = "Restore backup", &gotdbot.ButtonStyleSuccess{}
	}
	return &gotdbot.ReplyMarkupInlineKeyboard{Rows: [][]gotdbot.InlineKeyboardButton{
		{button(text, "mongo:"+action+":confirm", style)},
		{button("Cancel", "mongo:cancel", nil)},
	}}
}

func databaseKeyboard(names []string, page int) *gotdbot.ReplyMarkupInlineKeyboard {
	pages := max(1, (len(names)+pageSize-1)/pageSize)
	page = max(0, min(page, pages-1))
	start, end := page*pageSize, min((page+1)*pageSize, len(names))
	var rows [][]gotdbot.InlineKeyboardButton
	for i := start; i < end; i++ {
		if (i-start)%2 == 0 {
			rows = append(rows, nil)
		}
		rows[len(rows)-1] = append(rows[len(rows)-1], button(names[i], "mongo:db:"+strconv.Itoa(i-start), &gotdbot.ButtonStylePrimary{}))
	}
	if pages > 1 {
		var nav []gotdbot.InlineKeyboardButton
		if page > 0 {
			nav = append(nav, button("Previous", "mongo:page:"+strconv.Itoa(page-1), nil))
		}
		nav = append(nav, gotdbot.InlineKeyboardButton{Text: fmt.Sprintf("%d / %d", page+1, pages), Style: &gotdbot.ButtonStyleDefault{}, Type: &gotdbot.InlineKeyboardButtonTypeDisabled{}})
		if page+1 < pages {
			nav = append(nav, button("Next", "mongo:page:"+strconv.Itoa(page+1), nil))
		}
		rows = append(rows, nav)
	}
	rows = append(rows, []gotdbot.InlineKeyboardButton{button("Back", "mongo:menu", nil), button("Cancel", "mongo:cancel", nil)})
	return &gotdbot.ReplyMarkupInlineKeyboard{Rows: rows}
}
