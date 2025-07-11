package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/wcharczuk/go-chart"
	"github.com/wcharczuk/go-chart/drawing"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type stock struct {
	client        *http.Client
	srv           *sheets.Service
	spreadsheetId string
	sheetName     string
	values        map[string]map[string]float64
	valuesOrder   []string
	chartPath     string
	tokenPath     string
}

func newStock() stock {
	return stock{
		spreadsheetId: "1T5fBStqddB1jGeaV97aNaxa2clDul-5mh3L1JeyoAmQ",
		sheetName:     "Finance",
		chartPath:     "assets/images/chart.png",
		tokenPath:     "internal/config/gen-lang-client-0978399676-5efcfe192b5b.json",
	}
}

func (stock *stock) register(bot *discordgo.Session) {
	// create google sheets client
	stock.createClient()

	// add handlers
	bot.AddHandler(stock.StockCommand)
}

func toFloat64(num string) (float64, error) {
	num = strings.ReplaceAll(num, ",", ".")
	float, err := strconv.ParseFloat(num, 64)

	if err != nil {
		return 0, err
	}

	return float, err
}

func (stock *stock) createClient() {
	ctx := context.Background()

	// 1. Read the JSON key file
	keyFile := stock.tokenPath // Path to your downloaded JSON
	jsonKey, err := os.ReadFile(keyFile)
	if err != nil {
		log.Fatalf("Unable to read service account key: %v", err)
	}

	// 2. Configure JWT credentials
	creds, err := google.JWTConfigFromJSON(
		jsonKey,
		sheets.SpreadsheetsScope, // Required scope for Sheets API
	)
	if err != nil {
		log.Fatalf("Unable to parse credentials: %v", err)
	}

	// 3. Create an authenticated HTTP client
	stock.client = creds.Client(ctx)

	var sErr error
	stock.srv, sErr = sheets.NewService(ctx, option.WithHTTPClient(stock.client))
	if sErr != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", sErr)
	}
}

func (stock *stock) createGraphImage(xValues []float64, yValues []float64, title string) {
	// Create the output file
	f, err := os.Create(stock.chartPath)
	if err != nil {
		log.Println("Error creating file:", err)
		return
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Println("Failed to close graph file: ", err)
		}
	}()

	oldXValue := ""

	// Create the chart
	graph := chart.Chart{
		Width:  1280,
		Height: 540,
		DPI:    120,
		Title:  title,
		TitleStyle: chart.Style{
			Show:        true,
			StrokeColor: drawing.ColorWhite,
			FontColor:   drawing.ColorWhite,
			//FontSize:    32,
		},
		Background: chart.Style{
			Show:      true,
			FillColor: drawing.Color{R: 30, G: 30, B: 30, A: 255},
		},
		Canvas: chart.Style{
			Show:      true,
			FillColor: drawing.Color{R: 30, G: 30, B: 30, A: 255},
		},
		XAxis: chart.XAxis{
			Style: chart.Style{
				Show:        true,
				StrokeColor: drawing.Color{R: 255, G: 255, B: 255, A: 255},
				FontColor:   drawing.Color{R: 255, G: 255, B: 255, A: 255},
				//FontSize:    18,
			},
			NameStyle:    chart.Style{FontColor: drawing.ColorWhite},
			Name:         "Time",
			TickPosition: chart.TickPositionUnderTick,
		},
		YAxis: chart.YAxis{
			Style: chart.Style{
				Show:        true,
				StrokeColor: drawing.ColorWhite,
				FillColor:   drawing.ColorWhite,
				FontColor:   drawing.ColorWhite,

				//FontSize:    18,
			},
			NameStyle: chart.Style{FontColor: drawing.ColorWhite},
			Name:      "Price (€)",
		},
		Series: []chart.Series{
			chart.ContinuousSeries{
				Style: chart.Style{
					Show:            true,
					StrokeColor:     drawing.Color{R: 141, G: 150, B: 84, A: 255},
					FillColor:       drawing.Color{R: 141, G: 150, B: 84, A: 255},
					TextLineSpacing: 50,
				},
				XValueFormatter: func(v interface{}) string {
					// Parse the input date
					t := time.Unix(0, int64(v.(float64)))

					// Get the current time
					now := time.Now()

					// Calculate differences
					years := now.Year() - t.Year()
					months := int(now.Month()) - int(t.Month())
					days := now.Day() - t.Day()

					// Adjust for negative month/day differences
					if days < 0 {
						// Subtract 1 month
						months--
						// Get number of days in previous month
						prevMonth := now.AddDate(0, -1, 0)
						days += time.Date(prevMonth.Year(), prevMonth.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
					}

					if months < 0 {
						years--
						months += 12
					}

					value := ""

					// Return formatted result
					if years > 0 {
						value = fmt.Sprintf("%dY", years)
					} else if months > 0 {
						value = fmt.Sprintf("%dM", months)
					} else {
						value = fmt.Sprintf("%dD", days)
					}

					if value == oldXValue {
						oldXValue = value
						return "..."
					}
					oldXValue = value
					return value

				},
				XValues: xValues, // 0,1,2,3,4,5,6,7,8,...
				YValues: yValues, // Prices
			},
		},
	}

	// Render the chart as PNG
	err = graph.Render(chart.PNG, f)
	if err != nil {
		log.Println("Error rendering chart:", err)
		return
	}
}

// Update a cell value
func (stock *stock) setValue(cell string, value string) error {
	// Set timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Define the new value
	values := [][]interface{}{{value}}
	rb := &sheets.ValueRange{
		Values: values,
	}

	// Update the cell
	_, err := stock.srv.Spreadsheets.Values.Update(stock.spreadsheetId, cell, rb).
		ValueInputOption("RAW").
		Context(ctx).
		Do()

	if err != nil {
		return fmt.Errorf("unable to update value: %v", err)
	}

	return nil
}

func (stock *stock) getValues(cell string) (*sheets.ValueRange, error) {
	// Set timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// query value
	resp, err := stock.srv.Spreadsheets.Values.Get(stock.spreadsheetId, cell).
		Context(ctx).
		ValueRenderOption("FORMATTED_VALUE").
		DateTimeRenderOption("FORMATTED_STRING").
		Do()

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (stock *stock) getStockInfo(stockName string) error {
	// set stock name in sheets to later update values
	vErr := stock.setValue(stock.sheetName+"!C1", stockName)

	if vErr != nil {
		log.Println("Failed to set values: ", vErr)
		return vErr
	}

	// get data
	valuesRange := stock.sheetName + "!B1:B18"
	resp, err := stock.getValues(valuesRange)

	if err != nil {
		log.Printf("Unable to retrieve data from sheet: %v", err)
		return err
	}

	// process data and convert to ordered map using map + slice
	data := resp.Values

	stock.values = map[string]map[string]float64{}
	stock.valuesOrder = []string{"Today", "5 Days", "1 Month", "3 Month", "6 Months", "1 Year"}
	orderIndex := 0
	valueOrder := []string{"price", "changepct", "change"}

	zeroCounter := 0 // how many times values is 0 to check if every value is 0

	for i := 0; i < len(data); i++ {
		// create map if not exists
		if _, exists := stock.values[stock.valuesOrder[orderIndex]]; !exists {
			stock.values[stock.valuesOrder[orderIndex]] = map[string]float64{}
		}

		value, err := toFloat64(data[i][0].(string))
		if err != nil {
			// change valuesOrder every 3 values
			if (i+1)%3 == 0 {
				orderIndex++
			}
			zeroCounter++
			continue
		}
		// add data to stock values (embed)
		stock.values[stock.valuesOrder[orderIndex]][valueOrder[i%3]] = value

		// change valuesOrder every 3 values
		if (i+1)%3 == 0 {
			orderIndex++
		}
	}

	if zeroCounter == len(data) {
		err := fmt.Errorf("data is all 0 (stock was not found)")
		return err
	}

	// get chart data
	historyRange := stock.sheetName + "!D3:E"
	histResp, histErr := stock.getValues(historyRange)

	if histErr != nil {
		log.Fatalf("Unable to retrieve history data from sheet: %v", histErr)
		return histErr
	}

	// convert chart data
	chartData := []float64{}
	xTimestamps := []float64{}
	timeLayout := "2006-01-02"

	histValues := histResp.Values

	for i := 0; i < len(histValues); i++ {
		// price data
		value, err := toFloat64(histValues[i][1].(string))
		if err != nil {
			log.Println("Failed converting to float: ", err)
			return err
		}
		chartData = append(chartData, value)

		// time data
		t, err := time.Parse(timeLayout, histValues[i][0].(string))

		if err != nil {
			log.Println("Error converting timestamp: ", err)
		}

		xTimestamps = append(xTimestamps, float64(t.UnixNano()))
	}

	stock.createGraphImage(xTimestamps, chartData, stockName)

	return nil
}

func (stock *stock) generateFields() []*discordgo.MessageEmbedField {
	fields := []*discordgo.MessageEmbedField{}

	for _, time := range stock.valuesOrder {
		values := stock.values[time]
		field := discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("__%s:__ %.2f€", time, values["price"]),
			Value:  fmt.Sprintf("%+.2f€\n*(%+.2f%%)*", values["change"], values["changepct"]),
			Inline: true,
		}

		fields = append(fields, &field)
	}
	return fields
}

func (stock *stock) StockCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()

	if data.Name != "rheinmetall" && data.Name != "stock" {
		return
	}

	var (
		stockName    string
		title        string
		description  string
		color        string
		thumbnailUrl string
		embed        *discordgo.MessageEmbed
	)

	color = "8D9654"

	switch data.Name {
	case "rheinmetall":
		stockName = "RHM"
		title = "<a:FUERDIENATOINDENTOD:1346595146321625098> Rheinmetall Aktie - %.2f€ <a:FUERDIENATOINDENTOD:1346595146321625098>"
		description = "<a:Praying:1345448430499135560> für die NATO in den Tod <a:Praying:1345448430499135560>"
		thumbnailUrl = "https://media.discordapp.net/stickers/1346421311051927655.png?size=512&quality=lossless"
	case "stock":
		stockName = strings.ToUpper(data.Options[0].StringValue())
		title = "<:bakedStonksSchmied:1356396172285186179> " + stockName + " Aktie - %.2f€ <:bakedStonksSchmied:1356396172285186179>"
		description = "<:BakedBusinessSchmied:1356396420973989978> investiert fleißig <:BakedBusinessSchmied:1356396420973989978>"
		thumbnailUrl = ""
		if strings.ToUpper(data.Options[0].StringValue()) == "RHM" {
			stockName = "RHM"
			title = "<a:FUERDIENATOINDENTOD:1346595146321625098> Rheinmetall Aktie - %.2f€ <a:FUERDIENATOINDENTOD:1346595146321625098>"
			description = "<a:Praying:1345448430499135560> für die NATO in den Tod <a:Praying:1345448430499135560>"
			thumbnailUrl = "https://media.discordapp.net/stickers/1346421311051927655.png?size=512&quality=lossless"
		}
	default:
		return
	}

	// send typing...
	rErr := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if rErr != nil {
		log.Println("Failed to send stock interaction response: ", rErr)
	}

	err := stock.getStockInfo(stockName)

	if err != nil {
		content := fmt.Sprintf("Either this stock does not exist or there was an error fetching it: %s", err)
		_, rErr := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		})
		if rErr != nil {
			log.Println("Failed to send interaction response: ", rErr)
		}
		return
	}

	// adjust title
	title = fmt.Sprintf(title, stock.values["Today"]["price"])

	// get chart image
	file, fErr := os.Open(stock.chartPath)

	if fErr != nil {
		log.Println(fErr)
	}

	fileName := strings.ReplaceAll(stockName, ":", "")

	embed = &discordgo.MessageEmbed{
		Type:        discordgo.EmbedTypeImage,
		Title:       title,
		Description: description,
		Fields:      stock.generateFields(),
		Color:       convertHexColorToInt(color),
		Thumbnail:   &discordgo.MessageEmbedThumbnail{URL: thumbnailUrl},
		Image: &discordgo.MessageEmbedImage{
			URL: "attachment://" + fileName + ".png",
		},
	}

	_, responseErr := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
		Files: []*discordgo.File{
			{
				Name:        fileName + ".png",
				Reader:      file,
				ContentType: "image/png",
			},
		},
	})
	if responseErr != nil {
		log.Println("Failed to send stock interaction response: ", rErr)
	}
}
