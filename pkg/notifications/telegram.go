package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"
)

// TelegramClient handles sending notifications to Telegram
type TelegramClient struct {
	BotToken string
	ChatID   string
	Enabled  bool
}

// NewTelegramClient creates a new Telegram client
func NewTelegramClient(botToken, chatID string, enabled bool) *TelegramClient {
	return &TelegramClient{
		BotToken: botToken,
		ChatID:   chatID,
		Enabled:  enabled,
	}
}

// SendMessage sends a plain text message to Telegram
func (t *TelegramClient) SendMessage(message string) error {
	if !t.Enabled || t.BotToken == "" || t.ChatID == "" {
		return nil // Silently ignore if Telegram is not configured
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.BotToken)

	payload := map[string]interface{}{
		"chat_id":                  t.ChatID,
		"text":                     message,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal telegram payload: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned non-OK status: %d", resp.StatusCode)
	}

	return nil
}

// SendTokenClaimedNotification notifies about successfully claimed tokens
func (t *TelegramClient) SendTokenClaimedNotification(tokenName, tokenSymbol, amount, usdValue, txID string) {
	// Convert amount to a number, divide by 10^9 and format
	amountFloat, _ := strconv.ParseFloat(amount, 64)
	formattedAmount := fmt.Sprintf("%.2f", amountFloat/1e9)

	// Format USD value with 2 decimal places
	usdFloat, _ := strconv.ParseFloat(usdValue, 64)
	formattedUsd := fmt.Sprintf("%.2f", usdFloat)

	// Format the message with emojis
	message := fmt.Sprintf(
		"🎉 <b>Token Claimed Successfully!</b> 🎉\n\n"+
			"🪙 <b>Token:</b> %s (%s)\n"+
			"💰 <b>Amount:</b> %s\n"+
			"💵 <b>USD Value:</b> $%s\n"+
			"🕒 <b>Time:</b> %s\n"+
			"🔗 <b>Transaction:</b> <a href=\"https://solscan.io/tx/%s\">View on Solscan</a>",
		tokenName, tokenSymbol, formattedAmount, formattedUsd,
		time.Now().Format("2006-01-02 15:04:05"),
		txID,
	)

	if err := t.SendMessage(message); err != nil {
		log.Printf("Failed to send token claimed notification: %v", err)
	}
}

// SendTokenSoldNotification notifies about successfully sold tokens
func (t *TelegramClient) SendTokenSoldNotification(tokenName, tokenSymbol, amount, totalProfit string, profitSummary *ProfitSummary, solPrice float64, txID string) {
	// Convert amount to a number, divide by 10^9 and format
	amountFloat, _ := strconv.ParseFloat(amount, 64)
	formattedAmount := fmt.Sprintf("%.2f", amountFloat/1e9)

	// Parse the profit value
	profitFloat, _ := strconv.ParseFloat(totalProfit, 64)

	// Calculate USD equivalent of profit
	profitUsd := profitFloat * solPrice

	// Format the message with emojis
	message := fmt.Sprintf(
		"💎 <b>Transaction Complete!</b> 💎\n\n"+
			"🪙 <b>Token:</b> %s (%s)\n"+
			"💰 <b>Amount Sold:</b> %s\n"+
			"✨ <b>Net Profit:</b> %.5f SOL ($%.2f)\n"+
			"🕒 <b>Time:</b> %s\n"+
			"🔗 <b>Transaction:</b> <a href=\"https://solscan.io/tx/%s\">View on Solscan</a>",
		tokenName, tokenSymbol, formattedAmount,
		profitFloat, profitUsd,
		time.Now().Format("2006-01-02 15:04:05"),
		txID,
	)

	// Add profit summary if available
	if profitSummary != nil {
		// Calculate USD equivalents
		profit24hUsd := profitSummary.Last24h * solPrice
		profitWeekUsd := profitSummary.LastWeek * solPrice
		projectedWeekUsd := profitSummary.ProjectedWeek * solPrice

		summaryText := fmt.Sprintf(
			"\n\n📈 <b>Profit Summary:</b>\n"+
				"• <b>Last 24h:</b> %.5f SOL ($%.2f)\n"+
				"• <b>Last week:</b> %.5f SOL ($%.2f)\n"+
				"• <b>Projected weekly:</b> %.5f SOL ($%.2f)",
			profitSummary.Last24h, profit24hUsd,
			profitSummary.LastWeek, profitWeekUsd,
			profitSummary.ProjectedWeek, projectedWeekUsd,
		)

		message += summaryText
	}

	if err := t.SendMessage(message); err != nil {
		log.Printf("Failed to send token sold notification: %v", err)
	}
}

// SendTokenSaleErrorNotification notifies about failures when selling tokens
func (t *TelegramClient) SendTokenSaleErrorNotification(tokenName, tokenSymbol, amount, usdValue, errorMessage string, attempts int) {
	// Convert amount to a number, divide by 10^9 and format
	amountFloat, _ := strconv.ParseFloat(amount, 64)
	formattedAmount := fmt.Sprintf("%.2f", amountFloat/1e9)

	// Format USD value with 2 decimal places
	usdFloat, _ := strconv.ParseFloat(usdValue, 64)
	formattedUsd := fmt.Sprintf("%.2f", usdFloat)

	// Format the message with emojis
	message := fmt.Sprintf(
		"❌ <b>Token Sale Failed!</b> ❌\n\n"+
			"🪙 <b>Token:</b> %s (%s)\n"+
			"💰 <b>Amount:</b> %s\n"+
			"💵 <b>USD Value:</b> $%s\n"+
			"🔄 <b>Attempts:</b> %d\n"+
			"⚠️ <b>Error:</b> %s\n"+
			"🕒 <b>Time:</b> %s",
		tokenName, tokenSymbol, formattedAmount, formattedUsd, attempts,
		errorMessage,
		time.Now().Format("2006-01-02 15:04:05"),
	)

	if err := t.SendMessage(message); err != nil {
		log.Printf("Failed to send token sale error notification: %v", err)
	}
}

// FormatTokenAmount formats a token amount with appropriate decimal places
func FormatTokenAmount(amount float64, decimals int) string {
	// Always divide by 10^9 to show whole tokens for Solana tokens
	amount = amount / math.Pow10(9)

	// Format with 2 decimal places
	formatted := fmt.Sprintf("%.2f", amount)

	return formatted
}

// SendWelcomeMessage sends an initial welcome message with bot information and settings
func (t *TelegramClient) SendWelcomeMessage(walletAddress string, minimumUsdThreshold float64, checkInterval time.Duration) {
	// Format the welcome message with emojis and bot information
	message := fmt.Sprintf(
		"👋 <b>Welcome to Boop Airdrop Redeemer Bot!</b> 👋\n\n"+
			"🤖 <b>About this bot:</b>\n"+
			"This bot automatically monitors the Boop platform for valuable airdrops, "+
			"claims them when they meet your value threshold, and instantly converts tokens to SOL.\n\n"+
			"⚙️ <b>Current Settings:</b>\n"+
			"🔍 <b>Wallet:</b> %s\n"+
			"💵 <b>Minimum USD threshold:</b> $%.2f\n"+
			"⏱️ <b>Check interval:</b> %s\n"+
			"🔄 <b>Auto-sell to SOL:</b> Enabled\n"+
			"📊 <b>Token tracking:</b> Enabled\n\n"+
			"🔔 <b>Notifications:</b>\n"+
			"• Token claim success\n"+
			"• Token sale success\n"+
			"• Token sale errors\n\n"+
			"🚀 <b>Bot is now running!</b> You'll receive notifications automatically.",
		walletAddress,
		minimumUsdThreshold,
		checkInterval.String(),
	)

	if err := t.SendMessage(message); err != nil {
		log.Printf("Failed to send welcome message: %v", err)
	}
}

// SendHelpMessage sends detailed information about the bot's commands and settings
func (t *TelegramClient) SendHelpMessage() {
	// Format the help message with emojis and detailed information
	message := fmt.Sprintf(
		"📚 <b>Boop Airdrop Redeemer Bot Help</b> 📚\n\n" +
			"<b>What this bot does:</b>\n" +
			"• Monitors Boop platform for new airdrops\n" +
			"• Tracks token values over time\n" +
			"• Automatically claims airdrops meeting your value threshold\n" +
			"• Instantly sells tokens for SOL to lock in value\n" +
			"• Sends notifications on important events\n\n" +

			"<b>Configuration:</b>\n" +
			"These settings can be adjusted in your run scripts:\n\n" +

			"<code>WALLET_ADDRESS</code> - Your Solana wallet address\n" +
			"<code>WALLET_PRIVATE_KEY</code> - Private key for transactions\n" +
			"<code>MINIMUM_USD_THRESHOLD</code> - Minimum $ value to claim\n" +
			"<code>CHECK_INTERVAL</code> - How often to check (e.g. 1m)\n" +
			"<code>TELEGRAM_BOT_TOKEN</code> - This bot's token\n" +
			"<code>TELEGRAM_CHAT_ID</code> - Your chat ID\n" +
			"<code>ENABLE_TELEGRAM</code> - Toggle notifications\n\n" +

			"<b>Telegram Notifications:</b>\n" +
			"• <b>Welcome message:</b> Sent when bot starts\n" +
			"• <b>Token claimed:</b> When airdrop is successfully claimed\n" +
			"• <b>Token sold:</b> When tokens are converted to SOL\n" +
			"• <b>Error reports:</b> When token sales fail\n\n" +

			"<b>Support:</b>\n" +
			"For issues or questions, please check the GitHub repository documentation.",
	)

	if err := t.SendMessage(message); err != nil {
		log.Printf("Failed to send help message: %v", err)
	}
}

// SendStatusMessage sends a status update about the bot's operation
func (t *TelegramClient) SendStatusMessage(claimedCount int, scannedCount int, uptime time.Duration, lastCheckTime time.Time) {
	// Format the status message
	message := fmt.Sprintf(
		"📊 <b>Bot Status Update</b> 📊\n\n"+
			"⏱️ <b>Uptime:</b> %s\n"+
			"🔍 <b>Airdrops scanned:</b> %d\n"+
			"✅ <b>Airdrops claimed:</b> %d\n"+
			"🕒 <b>Last check:</b> %s\n\n"+
			"🤖 <b>System:</b> Running normally\n"+
			"🔄 <b>Next check:</b> In progress...",
		formatDuration(uptime),
		scannedCount,
		claimedCount,
		lastCheckTime.Format("2006-01-02 15:04:05"),
	)

	if err := t.SendMessage(message); err != nil {
		log.Printf("Failed to send status message: %v", err)
	}
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// ProfitSummary contains summary profit statistics
type ProfitSummary struct {
	Last24h       float64 // Profit in SOL for last 24 hours
	LastWeek      float64 // Profit in SOL for last week
	ProjectedWeek float64 // Projected weekly profit based on recent performance
}
