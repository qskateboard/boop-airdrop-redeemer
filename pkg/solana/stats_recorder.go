package solana

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TransactionType represents the type of transaction
type TransactionType string

const (
	// TypeClaim represents a claim transaction
	TypeClaim TransactionType = "CLAIM"
	// TypeSwap represents a swap/sell transaction
	TypeSwap TransactionType = "SWAP"
)

// TransactionStats stores statistics for a transaction
type TransactionStats struct {
	Timestamp   time.Time
	TokenSymbol string
	TokenAmount string
	Expenses    uint64 // in lamports
	GrossProfit uint64 // in lamports (for swaps)
	NetProfit   uint64 // gross profit - expenses (for swaps)
	TxHash      string
	TxType      TransactionType
}

// ProfitSummary contains summary profit statistics
type ProfitSummary struct {
	Last24h       float64 // Profit in SOL for last 24 hours
	LastWeek      float64 // Profit in SOL for last week
	ProjectedWeek float64 // Projected weekly profit based on recent performance
}

// StatsRecorder handles recording transaction statistics
type StatsRecorder struct {
	dataDir string
	mu      sync.Mutex
}

// NewStatsRecorder creates a new statistics recorder
func NewStatsRecorder(dataDir string) (*StatsRecorder, error) {
	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	return &StatsRecorder{
		dataDir: dataDir,
	}, nil
}

// RecordClaimStats records statistics for a claim transaction
func (s *StatsRecorder) RecordClaimStats(tokenSymbol, tokenAmount string, fees uint64, txHash string) error {
	return s.recordStats(TransactionStats{
		Timestamp:   time.Now(),
		TokenSymbol: tokenSymbol,
		TokenAmount: tokenAmount,
		Expenses:    fees,
		GrossProfit: 0,
		NetProfit:   0,
		TxHash:      txHash,
		TxType:      TypeClaim,
	})
}

// RecordSwapStats records statistics for a swap transaction
func (s *StatsRecorder) RecordSwapStats(tokenSymbol, tokenAmount string, fees, earnings uint64, txHash string) error {
	netProfit := int64(earnings) - int64(fees)
	if netProfit < 0 {
		netProfit = 0
	}

	return s.recordStats(TransactionStats{
		Timestamp:   time.Now(),
		TokenSymbol: tokenSymbol,
		TokenAmount: tokenAmount,
		Expenses:    fees,
		GrossProfit: earnings,
		NetProfit:   uint64(netProfit),
		TxHash:      txHash,
		TxType:      TypeSwap,
	})
}

// CalculateNetProfitFromClaimAndSwap calculates the net profit from a claim+swap transaction pair
func (s *StatsRecorder) CalculateNetProfitFromClaimAndSwap(claimFees, swapFees, swapEarnings uint64) float64 {
	// Calculate net profit in lamports (earnings - all fees)
	netProfitLamports := int64(swapEarnings) - int64(claimFees) - int64(swapFees)
	if netProfitLamports < 0 {
		netProfitLamports = 0
	}

	// Convert to SOL
	return float64(netProfitLamports) / 1_000_000_000
}

// GetProfitSummary calculates profit statistics for different time periods
func (s *StatsRecorder) GetProfitSummary() (ProfitSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	summary := ProfitSummary{}

	// Get current time for calculations
	now := time.Now()

	// Threshold times
	last24h := now.Add(-24 * time.Hour)
	lastWeek := now.Add(-7 * 24 * time.Hour)

	// Tracking vars
	var profit24h, profitWeek float64
	var recentTransactions int

	// Read transaction files
	files, err := s.getTransactionFiles()
	if err != nil {
		return summary, err
	}

	for _, file := range files {
		stats, err := s.readTransactionFile(file)
		if err != nil {
			continue
		}

		// Process each transaction
		for _, stat := range stats {
			// Convert net profit to SOL
			netProfitSol := float64(stat.NetProfit) / 1_000_000_000

			// Only count swap transactions for profit
			if stat.TxType == TypeSwap {
				// Last 24 hours
				if stat.Timestamp.After(last24h) {
					profit24h += netProfitSol
					recentTransactions++
				}

				// Last week
				if stat.Timestamp.After(lastWeek) {
					profitWeek += netProfitSol
				}
			}
		}
	}

	summary.Last24h = profit24h
	summary.LastWeek = profitWeek

	// Calculate projected weekly profit based on recent 24h
	if recentTransactions > 0 {
		// Project based on 24h activity
		summary.ProjectedWeek = profit24h * 7
	} else if profitWeek > 0 {
		// If no recent activity, use the actual weekly data
		summary.ProjectedWeek = profitWeek
	}

	return summary, nil
}

// getTransactionFiles returns a list of transaction file paths
func (s *StatsRecorder) getTransactionFiles() ([]string, error) {
	// Get all CSV files in the data directory
	pattern := filepath.Join(s.dataDir, "transactions_*.csv")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to find transaction files: %w", err)
	}
	return matches, nil
}

// readTransactionFile reads and parses a transaction file
func (s *StatsRecorder) readTransactionFile(filePath string) ([]TransactionStats, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open transaction file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV data: %w", err)
	}

	var stats []TransactionStats

	// Skip header row
	if len(records) > 1 {
		for i := 1; i < len(records); i++ {
			record := records[i]
			if len(record) < 8 {
				continue
			}

			timestamp, err := time.Parse(time.RFC3339, record[0])
			if err != nil {
				continue
			}

			// Parse expenses, gross, and net profit
			expenses := parseSOLToLamports(record[4])
			grossProfit := parseSOLToLamports(record[5])
			netProfit := parseSOLToLamports(record[6])

			stats = append(stats, TransactionStats{
				Timestamp:   timestamp,
				TxType:      TransactionType(record[1]),
				TokenSymbol: record[2],
				TokenAmount: record[3],
				Expenses:    expenses,
				GrossProfit: grossProfit,
				NetProfit:   netProfit,
				TxHash:      record[7],
			})
		}
	}

	return stats, nil
}

// parseSOLToLamports converts a SOL string value to lamports (uint64)
func parseSOLToLamports(solValue string) uint64 {
	// Remove any non-numeric characters except for the decimal point
	solValue = strings.TrimSpace(solValue)

	// Parse the float value
	value, err := strconv.ParseFloat(solValue, 64)
	if err != nil {
		return 0
	}

	// Convert to lamports (1 SOL = 1,000,000,000 lamports)
	return uint64(value * 1_000_000_000)
}

// recordStats writes statistics to the CSV file
func (s *StatsRecorder) recordStats(stats TransactionStats) error {
	if s == nil {
		return fmt.Errorf("stats recorder is not initialized")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Create filename based on year and month
	filename := fmt.Sprintf("transactions_%s.csv", time.Now().Format("2006-01"))
	filepath := filepath.Join(s.dataDir, filename)

	// Check if file exists
	fileExists := false
	if _, err := os.Stat(filepath); err == nil {
		fileExists = true
	}

	// Open file in append mode
	file, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open stats file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header if file is new
	if !fileExists {
		header := []string{
			"Timestamp", "Type", "Token", "Amount",
			"Expenses (SOL)", "Gross Profit (SOL)", "Net Profit (SOL)",
			"Transaction Hash",
		}
		if err := writer.Write(header); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
	}

	// Format values for CSV
	expensesSol := float64(stats.Expenses) / 1_000_000_000 // Convert lamports to SOL
	grossProfitSol := float64(stats.GrossProfit) / 1_000_000_000
	netProfitSol := float64(stats.NetProfit) / 1_000_000_000

	// Write record
	record := []string{
		stats.Timestamp.Format(time.RFC3339),
		string(stats.TxType),
		stats.TokenSymbol,
		stats.TokenAmount,
		fmt.Sprintf("%.9f", expensesSol),
		fmt.Sprintf("%.9f", grossProfitSol),
		fmt.Sprintf("%.9f", netProfitSol),
		stats.TxHash,
	}

	if err := writer.Write(record); err != nil {
		return fmt.Errorf("failed to write record: %w", err)
	}

	return nil
}
