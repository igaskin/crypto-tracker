package cmd

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/igaskin/crypto-tracker/lib"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/sheets/v4"
)

func NewImportCommand() *cobra.Command {
	var (
		accountID              string
		cryptoTransactionsFile string
		fiat                   string
		spreadsheetID          string
		spreadSheetName        string
	)
	const (
		defaultSpreadsheetName = "ROI"
		defaultExplorer        = "https://crypto.org/explorer/api/v1/"
	)
	var command = &cobra.Command{
		Use:   "import",
		Short: "Import crypto transaction csv data into google sheets",
		Run: func(cmd *cobra.Command, args []string) {
			importer := NewTransactionImporter(TransactionImporterOpts{
				Credentials:            "credentials.json",
				SpreadsheetID:          spreadsheetID,
				SheetName:              spreadSheetName,
				CryptoTransactionsFile: cryptoTransactionsFile,
				StartRow:               1,
				StartColumn:            "A", // TODO(igaskin): fix bugs so that this can be something other than "A"
				Fiat:                   fiat,
			})
			if err := importer.Validate(); err != nil {
				log.Fatal(err)
			}
			if err := importer.parseTransations(); err != nil {
				log.Fatalf("failed to import transactions; %v", err)
			}
			if accountID != "" {
				client := lib.NewExplorerClient(defaultExplorer)
				ctx := context.Background()
				resp, err := client.GetAccount(ctx, &lib.GetAccountOpts{
					AccountID: accountID,
				})
				if err != nil {
					log.Fatalf("failed to get account; %v", err)
				}
				fmt.Printf("total balance: %s\nusable balance: %s\ntotal rewards: %s\n",
					resp.Result.Totalbalance[0].Amount,
					resp.Result.Balance[0].Amount,
					resp.Result.Totalrewards[0].Amount,
				)
			}
		},
	}

	// TODO(igaskin): read these in from the ~/.crypto-tracker config file

	// the fiat type should be implied from the transactions file
	command.Flags().StringVar(&fiat, "fiat", "USD", "type of fiat to use (USD or EUR")
	command.Flags().StringVarP(&cryptoTransactionsFile, "file", "f", "crypto_transations.csv", "cyrpto transactions csv file")
	command.Flags().StringVarP(&spreadsheetID, "spreadsheet-id", "s", "", "id of google sheet (found in the URL)")
	command.Flags().StringVarP(&spreadSheetName, "spreadsheet-name", "n", defaultSpreadsheetName, "name of google sheet")
	command.Flags().StringVarP(&accountID, "account-id", "a", "", "cyrpto.org account id")
	return command
}

type PurchaseEvent string
type EarnEvent string

func (p PurchaseEvent) String() string {
	return string(p)
}

func (e EarnEvent) String() string {
	return string(e)
}

const (
	reoccurringBuy PurchaseEvent = "Recurring Buy"
	usdToCRO       PurchaseEvent = "USD -> CRO"
	eurToCRO       PurchaseEvent = "EUR -> CRO"
	buyCRO         PurchaseEvent = "Buy CRO"
	signupBonus    EarnEvent     = "Sign-up Bonus Unlocked"
	cryptoEarn     EarnEvent     = "Crypto Earn"
)

func (t *TransactionImporter) Validate() error {
	if t.SpreadsheetID == "" {
		return errors.New("Missing spreadsheet-id")
	}
	return nil
}

func (t *TransactionImporter) getSheetID() error {
	sheet, err := t.Googlesheet.Spreadsheets.Get(t.SpreadsheetID).Fields(googleapi.Field("sheets.properties")).Context(context.Background()).Do()
	if err != nil {
		return nil
	}
	for _, sheet := range sheet.Sheets {
		if sheet.Properties.Title == t.sheetName {
			t.sheetID = sheet.Properties.SheetId
		}
	}
	return nil
}

// TODO(igaskin): combine this with writeRow()
func (t *TransactionImporter) writeRowData(rowData []interface{}) error {
	var vr sheets.ValueRange
	vr.Values = append(vr.Values, rowData)

	rangez := fmt.Sprintf("%s!%s%d", t.sheetName, t.startColumn, t.currentRow)
	t.currentRow += 1
	_, err := t.Googlesheet.Spreadsheets.Values.Update(t.SpreadsheetID, rangez, &vr).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		return err
	}
	return nil
}

func (t *TransactionImporter) parseTransations() error {
	// write the header
	header := []interface{}{t.fiat, "CRO", "CRO Price", "Percent Change", fmt.Sprintf("%s Change", t.fiat)}
	err := t.writeRowData(header)
	if err != nil {
		return err
	}
	for {
		// TODO remove
		// break
		// Read each record from csv
		record, err := t.CryptoTransactions.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		switch record[1] {
		case reoccurringBuy.String(), usdToCRO.String(), eurToCRO.String(), buyCRO.String():
			if err := t.writeRow(record); err != nil {
				return err
			}
		case signupBonus.String(), cryptoEarn.String():
			// TODO(igaskin): track these seperatly
		default:
			continue
		}
	}

	// TODO(igaskin): need a more intelligent way to increment t.startColumn
	// characters can be expressed as runes which are int32, which should be capable of aritmetic
	footer := []interface{}{
		fmt.Sprintf("=SUM(%s%d:%s%d)", t.startColumn, t.startRowIndex, t.startColumn, t.currentRow-1),
		fmt.Sprintf("=SUM(%s%d:%s%d)", "B", t.startRowIndex, "B", t.currentRow-1),
		fmt.Sprintf("=AVERAGE(%s%d:%s%d)", "C", t.startRowIndex, "C", t.currentRow-1),
		fmt.Sprintf("=MINUS(DIVIDE(SUM(%[1]s%[3]d,%[2]s%[3]d), ABS(%[1]s%[3]d)),1)", "A", "E", t.currentRow),
		fmt.Sprintf("=SUM(%s%d:%s%d)", "E", t.startRowIndex, "E", t.currentRow-1),
	}
	err = t.writeRowData(footer)
	if err != nil {
		return err
	}

	err = t.getSheetID()
	if err != nil {
		log.Fatal(err)
	}

	// format data for readability
	resp, err := t.Googlesheet.Spreadsheets.BatchUpdate(t.SpreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{
			{
				// format fiat as currency
				RepeatCell: &sheets.RepeatCellRequest{
					Range: &sheets.GridRange{
						StartColumnIndex: t.startColumnIndex,
						EndColumnIndex:   t.startColumnIndex + 1,
						StartRowIndex:    t.startRowIndex - 1,
						EndRowIndex:      t.currentRow,
						SheetId:          t.sheetID,
					},
					Cell: &sheets.CellData{
						UserEnteredFormat: &sheets.CellFormat{
							NumberFormat: &sheets.NumberFormat{
								Type: "CURRENCY",
							},
						},
					},
					Fields: "userEnteredFormat.numberFormat",
				},
			},
			{
				// format fiat as currency
				RepeatCell: &sheets.RepeatCellRequest{
					Range: &sheets.GridRange{
						StartColumnIndex: t.startColumnIndex + 4,
						EndColumnIndex:   t.startColumnIndex + 5,
						StartRowIndex:    t.startRowIndex - 1,
						EndRowIndex:      t.currentRow,
						SheetId:          t.sheetID,
					},
					Cell: &sheets.CellData{
						UserEnteredFormat: &sheets.CellFormat{
							NumberFormat: &sheets.NumberFormat{
								Type: "CURRENCY",
							},
						},
					},
					Fields: "userEnteredFormat.numberFormat",
				},
			},
			{
				// format cro purchase price as currency
				RepeatCell: &sheets.RepeatCellRequest{
					Range: &sheets.GridRange{
						StartColumnIndex: t.startColumnIndex + 2,
						EndColumnIndex:   t.startColumnIndex + 3,
						StartRowIndex:    t.startRowIndex - 1,
						EndRowIndex:      t.currentRow,
						SheetId:          t.sheetID,
					},
					Cell: &sheets.CellData{
						UserEnteredFormat: &sheets.CellFormat{
							NumberFormat: &sheets.NumberFormat{
								Type: "CURRENCY",
							},
						},
					},
					Fields: "userEnteredFormat.numberFormat",
				},
			},
			{
				// format CRO as a float
				RepeatCell: &sheets.RepeatCellRequest{
					Range: &sheets.GridRange{
						StartColumnIndex: t.startColumnIndex + 1,
						EndColumnIndex:   t.startColumnIndex + 2,
						StartRowIndex:    t.startRowIndex - 1,
						EndRowIndex:      t.currentRow,
						SheetId:          t.sheetID,
					},
					Cell: &sheets.CellData{
						UserEnteredFormat: &sheets.CellFormat{
							NumberFormat: &sheets.NumberFormat{
								Type:    "NUMBER",
								Pattern: "#,##0.00",
							},
						},
					},
					Fields: "userEnteredFormat.numberFormat",
				},
			},
			{
				// format change as percentage
				RepeatCell: &sheets.RepeatCellRequest{
					Range: &sheets.GridRange{
						StartColumnIndex: t.startColumnIndex + 3,
						EndColumnIndex:   t.startColumnIndex + 4,
						StartRowIndex:    t.startRowIndex - 1,
						EndRowIndex:      t.currentRow,
						SheetId:          t.sheetID,
					},
					Cell: &sheets.CellData{
						UserEnteredFormat: &sheets.CellFormat{
							NumberFormat: &sheets.NumberFormat{
								Type:    "PERCENT",
								Pattern: "#.0#%",
							},
						},
					},
					Fields: "userEnteredFormat.numberFormat",
				},
			},
			{
				// set font family
				RepeatCell: &sheets.RepeatCellRequest{
					Range: &sheets.GridRange{
						StartColumnIndex: t.startColumnIndex,
						EndColumnIndex:   t.startColumnIndex + 5,
						StartRowIndex:    t.startRowIndex - 1,
						EndRowIndex:      t.currentRow,
						SheetId:          t.sheetID,
					},
					Cell: &sheets.CellData{
						UserEnteredFormat: &sheets.CellFormat{
							TextFormat: &sheets.TextFormat{
								FontFamily: "Inconsolata",
								FontSize:   11,
							},
						},
					},
					Fields: "userEnteredFormat.textFormat",
				},
			},
			// clear any existing boarders
			{
				UpdateBorders: &sheets.UpdateBordersRequest{
					Range: &sheets.GridRange{
						StartColumnIndex: t.startColumnIndex,
						EndColumnIndex:   t.startColumnIndex + 5,
						StartRowIndex:    t.startRowIndex - 1,
						EndRowIndex:      t.currentRow,
						SheetId:          t.sheetID,
					},
					Top: &sheets.Border{
						Style: "NONE",
					},
					InnerHorizontal: &sheets.Border{
						Style: "NONE",
					},
					Bottom: &sheets.Border{
						Style: "NONE",
					},
					InnerVertical: &sheets.Border{
						Style: "NONE",
					},
					Left: &sheets.Border{
						Style: "NONE",
					},
					Right: &sheets.Border{
						Style: "NONE",
					},
				},
			},
			// add border to footer
			{
				UpdateBorders: &sheets.UpdateBordersRequest{
					Range: &sheets.GridRange{
						StartColumnIndex: t.startColumnIndex,
						EndColumnIndex:   t.startColumnIndex + 5,
						StartRowIndex:    t.currentRow - 2,
						EndRowIndex:      t.currentRow - 1,
						SheetId:          t.sheetID,
					},
					Top: &sheets.Border{
						Style: "SOLID",
						Color: &sheets.Color{
							Red:   0,
							Green: 0,
							Blue:  0,
						},
					},
				},
			},
			// add border to header
			{
				UpdateBorders: &sheets.UpdateBordersRequest{
					Range: &sheets.GridRange{
						StartColumnIndex: t.startColumnIndex,
						EndColumnIndex:   t.startColumnIndex + 5,
						StartRowIndex:    t.startRowIndex - 1,
						EndRowIndex:      t.startRowIndex,
						SheetId:          t.sheetID,
					},
					Bottom: &sheets.Border{
						Style: "SOLID",
						Color: &sheets.Color{
							Red:   0,
							Green: 0,
							Blue:  0,
						},
					},
				},
			},
			{
				// bold the summary row
				RepeatCell: &sheets.RepeatCellRequest{
					Range: &sheets.GridRange{
						StartColumnIndex: t.startColumnIndex,
						EndColumnIndex:   t.startColumnIndex + 5,
						StartRowIndex:    t.currentRow - 2,
						EndRowIndex:      t.currentRow - 1,
						SheetId:          t.sheetID,
					},
					Cell: &sheets.CellData{
						UserEnteredFormat: &sheets.CellFormat{
							TextFormat: &sheets.TextFormat{
								Bold: true,
							},
						},
					},
					Fields: "userEnteredFormat.textFormat",
				},
			},
			{
				// conditional formatting gains/losses
				AddConditionalFormatRule: &sheets.AddConditionalFormatRuleRequest{
					Index: 0,
					Rule: &sheets.ConditionalFormatRule{
						Ranges: []*sheets.GridRange{
							{
								StartColumnIndex: t.startColumnIndex + 3,
								EndColumnIndex:   t.startColumnIndex + 5,
								StartRowIndex:    t.startRowIndex - 1,
								EndRowIndex:      t.currentRow,
								SheetId:          t.sheetID,
							},
						},
						BooleanRule: &sheets.BooleanRule{
							Condition: &sheets.BooleanCondition{
								Type: "NUMBER_GREATER",
								Values: []*sheets.ConditionValue{
									{
										UserEnteredValue: "0",
									},
								},
							},
							Format: &sheets.CellFormat{
								BackgroundColor: &sheets.Color{
									Red:   0.850,
									Green: 0.917,
									Blue:  0.827,
								},
							},
						},
					},
				},
			},
			{
				// conditional formatting gains/losses
				AddConditionalFormatRule: &sheets.AddConditionalFormatRuleRequest{
					Index: 0,
					Rule: &sheets.ConditionalFormatRule{
						Ranges: []*sheets.GridRange{
							{
								StartColumnIndex: t.startColumnIndex + 3,
								EndColumnIndex:   t.startColumnIndex + 5,
								StartRowIndex:    t.startRowIndex - 1,
								EndRowIndex:      t.currentRow,
								SheetId:          t.sheetID,
							},
						},
						BooleanRule: &sheets.BooleanRule{
							Condition: &sheets.BooleanCondition{
								Type: "NUMBER_LESS_THAN_EQ",
								Values: []*sheets.ConditionValue{
									{
										UserEnteredValue: "0",
									},
								},
							},
							Format: &sheets.CellFormat{
								BackgroundColor: &sheets.Color{
									Red:   0.956,
									Green: 0.8,
									Blue:  0.8,
								},
							},
						},
					},
				},
			},
		},
	}).Context(context.Background()).Do()
	if err != nil {
		return err
	}
	_ = resp
	return nil
}

type RowData struct {
	Fiat string
	CRO  string
	// TODO: compute these from the other two values
	PurchasePrice string
	PercentChange string
	FiatChange    string
}

func (r *RowData) ToSlice() []interface{} {
	e := reflect.ValueOf(r).Elem()
	s := make([]interface{}, e.NumField())
	for i := 0; i < e.NumField(); i++ {
		s[i] = e.Field(i).Interface()
	}
	return s
}

// TODO(igaskin): be a bro and make a go-coingecko client
func getCROPrice() float32 {
	type p struct {
		CryptoComChain struct {
			Usd float32 `json:"usd"`
		} `json:"crypto-com-chain"`
	}

	var price p

	resp, err := http.Get("https://api.coingecko.com/api/v3/simple/price?ids=crypto-com-chain&vs_currencies=usd")
	if err != nil {
		// handle err
		fmt.Println(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
	}
	err = json.Unmarshal(body, &price)
	if err != nil {
		fmt.Println(err)
	}
	return price.CryptoComChain.Usd
}

var croCurrentPrice = getCROPrice()

// TODO: refactor this to be part of the transaction importer struct
func NewRowData(record []string, rowNumber int64) *RowData {
	_, b, _, d, _, f, _, h, _, _ := record[0], record[1], record[2], record[3], record[4], record[5], record[6], record[7], record[8], record[9]

	rowData := &RowData{
		PurchasePrice: fmt.Sprintf("=(DIVIDE(A%[1]d,B%[1]d))", rowNumber),
		PercentChange: fmt.Sprintf("=(DIVIDE(MINUS(%[2]f,C%[1]d),%[2]f))", rowNumber, croCurrentPrice),
		FiatChange:    fmt.Sprintf("=MULTIPLY(A%[1]d,D%[1]d)", rowNumber),
	}
	switch b {
	case reoccurringBuy.String():
		rowData.Fiat, rowData.CRO = d, f
	case usdToCRO.String(), eurToCRO.String():
		rowData.Fiat, rowData.CRO = h, f
	case buyCRO.String():
		rowData.Fiat, rowData.CRO = h, d
	case signupBonus.String(), cryptoEarn.String():
	default:
		return nil
	}
	return rowData
}

func (t *TransactionImporter) writeRow(record []string) error {
	var err error
	var vr sheets.ValueRange
	vr.Values = append(vr.Values, NewRowData(record, t.currentRow).ToSlice())

	rangez := fmt.Sprintf("%s!%s%d", t.sheetName, t.startColumn, t.currentRow)
	t.currentRow += 1
	_, err = t.Googlesheet.Spreadsheets.Values.Update(t.SpreadsheetID, rangez, &vr).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		return err
	}
	fmt.Println(record)
	return nil
}

type TransactionImporter struct {
	Googlesheet        *sheets.Service
	SpreadsheetID      string
	CryptoTransactions *csv.Reader
	fiat               string
	currentRow         int64
	startRowIndex      int64
	startColumn        string
	startColumnIndex   int64
	sheetName          string
	sheetID            int64
}

type TransactionImporterOpts struct {
	Credentials            string
	SpreadsheetID          string
	CryptoTransactionsFile string
	StartRow               int64
	StartColumn            string
	SheetName              string
	Fiat                   string
}

func NewTransactionImporter(opts TransactionImporterOpts) *TransactionImporter {
	b, err := ioutil.ReadFile(opts.Credentials)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	googlesheet, err := sheets.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	csvfile, err := os.Open(opts.CryptoTransactionsFile)
	if err != nil {
		log.Fatalf("unable to open transactions file: %s", err)
	}
	cryptoTransactions := csv.NewReader(csvfile)
	startColumnIndex := []rune(strings.ToUpper(opts.StartColumn))[0] - 65

	return &TransactionImporter{
		Googlesheet:        googlesheet,
		SpreadsheetID:      opts.SpreadsheetID,
		CryptoTransactions: cryptoTransactions,
		fiat:               opts.Fiat,
		currentRow:         opts.StartRow,
		startRowIndex:      opts.StartRow,
		startColumn:        opts.StartColumn,
		startColumnIndex:   int64(startColumnIndex),
		sheetName:          opts.SheetName,
	}
}
