package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"gopkg.in/Iwark/spreadsheet.v2"
	"hash/crc32"
	"strings"
	"encoding/json"
	"io"
	"time"
	"bytes"
)

const PAUSE_DURATION int = 20

var LOG_FILE_PATH string = os.Getenv("LOG_FILE_PATH")
var PLAYRIX_SPREAD_SHEET_ID = os.Getenv("PLAYRIX_SPREAD_SHEET_ID")
var GOOGLE_CRED_PATH string = os.Getenv("GOOGLE_CRED_PATH")
var GREEDLY_API_KEY string = os.Getenv("GREEDLY_API_KEY")
var GREEDLY_DATABASE_ID string = os.Getenv("GREEDLY_DATABASE_ID")
var SHEET_NAMES = os.Getenv("SHEET_NAMES")

var eventLogger *log.Logger
var errorLogger *log.Logger
var logMutex sync.Mutex

// LogWithMutex logs messages with thread safety.
func LogWithMutex(eventLogger *log.Logger, message string) {
    logMutex.Lock()
    defer logMutex.Unlock()
    eventLogger.Println(message)
}

// Grid represents a grid in Gridly with ID and name.
type Grid struct {
    ID      string   `json:"id"`
    Name    string   `json:"name"`
}

// View represents a view associated with a grid in Gridly.
type View struct {
    ID      string   `json:"id"`
    Name    string   `json:"name"`
	GridId  string   `json:"gridId"`

}

// Record represents a row with cells in a grid view.
type Record struct {
    ID    string `json:"id"`
    Cells []Cell `json:"cells"`
}

// Cell represents a single cell in a row, with column ID and value.
type Cell struct {
    ColumnID string     `json:"columnId"`
    Value    string 	`json:"value"`
}


// RowChecksum represents a hashed row with ID, checksum, and content.
type RowChecksum struct {
	RowID int
	RowCheckSum uint32
	RowContent []string
}

// SheetChecksum represents a sheet with a title, reference ID, and hashed rows.
type SheetChecksum struct {
	SheetTitle string
	Reference string //for viewID
	HashedRows []RowChecksum
	
}





// GridlyClient handles API requests to Gridly.
type GridlyClient struct {
	APIKey     string
	HTTPClient *http.Client
	BaseURL    string
}

// NewGridlyClient creates and initializes a new Gridly API client.
func NewGridlyClient(apiKey string) *GridlyClient {
	return &GridlyClient{
		APIKey:     apiKey,
		HTTPClient: http.DefaultClient,
		BaseURL:    "https://api.gridly.com/v1",
	}
}

// executeAPIRequest performs an HTTP request and returns the response body or error.
func (c *GridlyClient) executeAPIRequest(method, endpoint string, body []byte) ([]byte, error) {
	url := fmt.Sprintf("%s%s", c.BaseURL, endpoint)
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Add("Authorization", "ApiKey "+c.APIKey)
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}
	return responseBody, nil
}

// AddRowsToGridly adds new rows to Gridly based on the provided RowChecksum data.
func (client *GridlyClient) AddRowsToGridly(viewID string, newRows []RowChecksum) error {
	var records []Record
	endpoint := fmt.Sprintf("/views/%s/records", viewID)
	fmt.Println("my viewID", viewID)
	for _, row := range newRows {
		recordID := row.RowContent[0] // the first value is recordID
		columnValues := row.RowContent[1:] 

		// Prepare cells with column IDs and values
		var cells []Cell
		for i, value := range columnValues {
			columnID := fmt.Sprintf("column%d", i+1)
			cells = append(cells, Cell{ColumnID: columnID, Value: value})
		}

		// Create record object for the new row
		record := Record{
			ID:    recordID,
			Cells: cells,
		}
		records = append(records, record)

	fmt.Println(records)
	jsonBody, err := json.Marshal(records)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %v", err)
	}

	_, err = client.executeAPIRequest("POST", endpoint, jsonBody); if err != nil {
		return fmt.Errorf("error adding new record: %v", err)
	}

	LogWithMutex(eventLogger, "New rows were added to Gridly.")
	}

	return nil
}

// UpdateGridlyRow updates rows in Gridly based on the provided RowHash data.
func (c *GridlyClient) UpdateGridlyRow(viewID string, newRows []RowChecksum) error {
	for _, row := range newRows {
		recordID := row.RowContent[0]
		columnValues := row.RowContent[1:]

		// Prepare cells with column IDs and values
		var cells []Cell
		for i, value := range columnValues {
			columnID := fmt.Sprintf("column%d", i+1)
			cells = append(cells, Cell{ColumnID: columnID, Value: value})
		}

		// Create record object for the row
		record := Record{
			ID:    recordID,
			Cells: cells,
		}

		jsonBody, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %v", err)
		}

		// Send PATCH request to update the record
		endpoint := fmt.Sprintf("/views/%s/records/%s", viewID, recordID)
		_, err = c.executeAPIRequest("PATCH", endpoint, jsonBody)
		if err != nil {
			return fmt.Errorf("error updating record %s: %v", recordID, err)
		}

		fmt.Printf("Successfully updated record %s\n", recordID)
	}
	return nil
}


// FetchGridsByDatabaseID fetches grids associated with a specific database
func (c *GridlyClient) FetchGridsByDatabaseID(databaseID string) ([]Grid, error) {
	endpoint := fmt.Sprintf("/grids?dbId=%s", databaseID)
	body, err := c.executeAPIRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var grids []Grid
	if err := json.Unmarshal(body, &grids); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}
	return grids, nil
}

// FetchGridView fetches a view associated with a given grid ID.
func (c *GridlyClient) FetchGridView(gridID string) (View, error) {
	endpoint := fmt.Sprintf("/views?gridId=%s", gridID)
	body, err := c.executeAPIRequest("GET", endpoint, nil)
	if err != nil {
		return View{}, err
	}

	var views []View
	if err := json.Unmarshal(body, &views); err != nil {
		return View{}, fmt.Errorf("error parsing JSON: %v", err)
	}

	if len(views) > 0 {
		return views[0], nil
	}
	return View{}, fmt.Errorf("no views found")
}

// FetchRecordsForView fetches records for a specific view ID.
func (c *GridlyClient) FetchRecordsForView(viewID string) ([]Record, error) {
	endpoint := fmt.Sprintf("/views/%s/records", viewID)
	body, err := c.executeAPIRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var records []Record
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	return records, nil
}

// GetSheetList splits a sheet list string into individual sheet names.
func GetSheetList(sheetsString string, separator string) ([]string, error) {
	if sheetsString == "" {
		return nil, fmt.Errorf("no sheets name in enviromant varaibles")
    }
	return strings.Split(sheetsString, separator), nil
}


// ComputeRowHashes hashes each row in a sheet for checksum validation.
func ComputeRowHashes(sh *spreadsheet.Sheet) []RowChecksum {
	rowHahes := make([]RowChecksum, 0, 10)

	for index, row := range sh.Rows {
		// Skip the header row
		if index == 0 {
			continue
		}
		stringRow := rowToStringSlice(row)
		rowCheckSum := hashRowCells(stringRow)
		rowHahes = append(rowHahes, RowChecksum{RowID: index-1, RowCheckSum: rowCheckSum, RowContent: stringRow})
	}

	return rowHahes
}

// hashRowCells computes a CRC32 checksum for concatenated cell values.
func hashRowCells(rowData []string) uint32 {

	//cellValues [GameTip_1 John Протираем пыль... Wiping the dust... 50 1 -]
	concatenatedValues := strings.Join(rowData, "")
	
	table := crc32.MakeTable(crc32.IEEE)
	checksum := crc32.Checksum([]byte(concatenatedValues), table)
	return checksum
}

// rowToStringSlice converts spreadsheet cells to a slice of their values.
func rowToStringSlice(rowCells []spreadsheet.Cell) []string {
	cellValues := make([]string, 0, 10)
	for _, cell := range rowCells {
		cellValues = append(cellValues, cell.Value)
	}
	return cellValues
}

// RecordsToRows converts a Record's cells to a row slice.
func RecordsToRows(record Record) []string {
	greedlyRows := make([]string, 0, 10)

	greedlyRows = append(greedlyRows, string(record.ID))
	for _, v := range record.Cells {
		greedlyRows = append(greedlyRows, string(v.Value))
	}
	return greedlyRows
}

// ProcessSheet processes each sheet by hashing rows and storing results in initSheetHash.
func ProcessSheet(title string, ss spreadsheet.Spreadsheet, wg *sync.WaitGroup, mu *sync.Mutex, initSheetHash map[string]SheetChecksum) {
	defer wg.Done()

	sheet, err := ss.SheetByTitle(title)
	if err != nil {
		logMessage := fmt.Sprintf("can't open sheet %v : %v", title, err)
		LogWithMutex(errorLogger, logMessage)
		return
	}

	hashedRows := ComputeRowHashes(sheet)
	sheetHash := SheetChecksum{HashedRows: hashedRows, SheetTitle: sheet.Properties.Title}

	// Lock to prevent concurrent write issues
	mu.Lock()
	initSheetHash[title] = sheetHash
	mu.Unlock()

	logMessage := fmt.Sprintf("all columns of sheet with title %s were hashed", title)
	LogWithMutex(eventLogger, logMessage)
}

// ProcessGridlyGrid processes each sheet by hashing rows and storing results in initSheetHash.
func ProcessGridlyGrid(grid Grid, gridlySheets map[string]SheetChecksum, client *GridlyClient, wg *sync.WaitGroup, mu *sync.Mutex) {
	defer wg.Done()

	gridlySheet := SheetChecksum{SheetTitle: grid.Name}
	view, err := client.FetchGridView(grid.ID)
	if err != nil {
		logText := fmt.Sprintf("Error fetching view for %v, error: %v", grid.ID, err)
		LogWithMutex(errorLogger, logText)
		return
	}
	gridlySheet.Reference = view.ID
	records, err := client.FetchRecordsForView(view.ID)
	if err != nil {
		logText := fmt.Sprintf("Error fetching records with viewID %v, error: %v", view.ID, err)
		LogWithMutex(errorLogger, logText)
		return
	}

	for id, record := range records {
		processedRows := RecordsToRows(record)
		gridlyRowHash := RowChecksum{RowID: id, RowCheckSum: hashRowCells(processedRows), RowContent: processedRows}

		gridlySheet.HashedRows = append(gridlySheet.HashedRows, gridlyRowHash)
	}

	mu.Lock()
	gridlySheets[grid.Name] = gridlySheet
	mu.Unlock()
}
//Check if Gridly sheet(Grid) equals Google Sheet sheet
func SheetsEqual(gridlyTitle string, hashedSheet SheetChecksum, googleSheets map[string]SheetChecksum) (bool, []RowChecksum) {

	var rowsToPush []RowChecksum

	value, exists := googleSheets[gridlyTitle]

	if !exists {
		return false, nil
	}

	for _, row := range value.HashedRows {
		if RowInGridly(hashedSheet.HashedRows, row) {
			continue
		} else {
			rowsToPush = append(rowsToPush, row)
		}
	}
	if len(rowsToPush) != 0 {
		return false, rowsToPush
	}
	return true, nil
	
}

// Check if Google wow match any Gridly row
func RowInGridly(gridlyRows []RowChecksum, googleRow RowChecksum) bool {
	for _, row := range gridlyRows {
		if row.RowCheckSum == googleRow.RowCheckSum && row.RowID == googleRow.RowID {
			return true
		}
	}
	return false
}

// FindNewRows identifies rows in currentGoogleHash not found in the corresponding Gridly sheet
func FindNewRows(currentGoogleHash map[string]SheetChecksum, gridlySheet SheetChecksum) []RowChecksum {
	value := currentGoogleHash[gridlySheet.SheetTitle]
	googleLen := len(value.HashedRows)
	gridlyLen := len(gridlySheet.HashedRows)
	dif := googleLen - gridlyLen

	if dif <= 0 {
		return nil
	} else { 
		return value.HashedRows[googleLen-dif:]
	}
}   

func main() {
    fmt.Println("The program has started...")

    // Set up log file for writing logs
    logFile, err := os.OpenFile(LOG_FILE_PATH, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
    if err != nil {
        log.Fatalf("Failed to open the log file: %v\n", err)
    }
    defer logFile.Close()

    // Initialize loggers for event and error logging
    eventLogger = log.New(logFile, "[EVENT]", log.LstdFlags)
    errorLogger = log.New(logFile, "[ERROR]", log.LstdFlags)

    // Retrieve the list of Google Sheet names from environment variables
    sheetNames, err := GetSheetList(SHEET_NAMES, ",")
    if err != nil {
        LogWithMutex(errorLogger, fmt.Sprintf("Error reading sheet titles from environment: %v", err))
    }

    LogWithMutex(eventLogger, fmt.Sprintf("Sheet titles to sync in GoogleSheet and Gridly: %v", sheetNames))

    // Initialize Google Sheets service
    service, err := spreadsheet.NewService()
    if err != nil {
        LogWithMutex(errorLogger, fmt.Sprintf("Failed to set up service with client_secret file: %v", err))
        os.Exit(1)
    }

    // Fetch the initial spreadsheet data
    ssheet, err := service.FetchSpreadsheet(PLAYRIX_SPREAD_SHEET_ID)
    if err != nil {
        LogWithMutex(errorLogger, fmt.Sprintf("Cannot open spreadsheet %s: %v", PLAYRIX_SPREAD_SHEET_ID, err))
        os.Exit(1)
    }

    // Initialize a map for storing checksums of each sheet
    initSheetHash := make(map[string]SheetChecksum)
    var wg sync.WaitGroup
    var mu sync.Mutex

    // Process each sheet concurrently to fetch initial data and calculate checksums
    for _, title := range sheetNames {
        wg.Add(1)
        go ProcessSheet(title, ssheet, &wg, &mu, initSheetHash)
    }
    wg.Wait()

    // Set up Gridly client
    client := NewGridlyClient(GREEDLY_API_KEY)
    gridlySheets := make(map[string]SheetChecksum)

    // Fetch Gridly grids for the given database ID
    grids, err := client.FetchGridsByDatabaseID(GREEDLY_DATABASE_ID)
    if err != nil {
        LogWithMutex(errorLogger, "Error fetching grids from Gridly")
        os.Exit(1)
    }

    // Process each Gridly grid concurrently and calculate checksums
    for _, grid := range grids {
        wg.Add(1)
        go ProcessGridlyGrid(grid, gridlySheets, client, &wg, &mu)
    }
    wg.Wait()

    LogWithMutex(eventLogger, "All initial sheets processed and checksums calculated.")

    // Compare Gridly sheets to initial Google Sheets, logging differences
    for k, v := range gridlySheets {
        same, _ := SheetsEqual(k, v, initSheetHash)
        if same {
            LogWithMutex(eventLogger, fmt.Sprintf("Greedly Sheet %s fully matches the initial Google Sheet", k))
        } else {
            LogWithMutex(eventLogger, fmt.Sprintf("Greedly Sheet %s does not match initial Google Sheet; updating Greedly", k))
        }
    }

    // Continuous loop to check for updates between Google Sheets and Gridly sheets
	for {
		LogWithMutex(eventLogger, "Starting a new Gridly synchronization check")
		time.Sleep(time.Duration(PAUSE_DURATION) * time.Second)
	
		currentGoogleHash := make(map[string]SheetChecksum)
		var wg sync.WaitGroup
		var mu sync.Mutex
		
		// Refetch the initial spreadsheet data
		ssheet, err := service.FetchSpreadsheet(PLAYRIX_SPREAD_SHEET_ID)
		if err != nil {
			LogWithMutex(errorLogger, fmt.Sprintf("Cannot open spreadsheet %s: %v", PLAYRIX_SPREAD_SHEET_ID, err))
			os.Exit(1)
		}
	
		// Recompute checksums for Google Sheets
		for _, title := range sheetNames {
			wg.Add(1)
			go ProcessSheet(title, ssheet, &wg, &mu, currentGoogleHash)
		}
		wg.Wait()
	
		// Compare updated Google Sheets to Gridly sheets and apply updates as necessary
		for k, v := range gridlySheets {
			same, rowsToPush := SheetsEqual(k, v, currentGoogleHash)
	
			// Check for rows that are in Google Sheets but not in Gridly
			newRows := FindNewRows(currentGoogleHash, v) 
			fmt.Println("These are new rows to add", newRows)
	
			fmt.Println("These rows to update:", rowsToPush) 
	
			if !same {
				LogWithMutex(eventLogger, fmt.Sprintf("Greedly Sheet %s needs update to match Google Sheet", k))
				if err := client.UpdateGridlyRow(v.Reference, rowsToPush); err != nil {
					LogWithMutex(errorLogger, fmt.Sprintf("Error updating rows: %v", err))
				}
			}
	
			// Add new rows to Gridly if they exist
			if len(newRows) > 0 {
				LogWithMutex(eventLogger, fmt.Sprintf("Adding new rows to Gridly Sheet %s", k))
				err := client.AddRowsToGridly(v.Reference, newRows); if err != nil {
					logText := fmt.Sprintf("Error adding new rows: %v", err)
					LogWithMutex(errorLogger, logText)
				}
			} else {
				LogWithMutex(eventLogger, fmt.Sprintf("Greedly Sheet %s fully matches the updated Google Sheet", k))
			}
		}
	
		// Update the Gridly sheets hash for the next cycle
		gridlySheets = currentGoogleHash

	}
}