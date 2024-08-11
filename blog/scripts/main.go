package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	exiftool "github.com/barasher/go-exiftool"
)

type ImageMetadata struct {
	Latitude  float64
	Longitude float64
	Altitude  string
	DateTime  time.Time
}

// convertDMSToDecimal converts a DMS (Degrees, Minutes, Seconds) coordinate to Decimal Degrees.
func convertDMSToDecimal(dms string) (float64, error) {
	parts := strings.Split(dms, " ")
	if len(parts) < 4 {
		return 0, fmt.Errorf("invalid DMS format: %s", dms)
	}

	degrees, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, err
	}

	minutes, err := strconv.ParseFloat(strings.TrimSuffix(parts[2], "'"), 64)
	if err != nil {
		return 0, err
	}

	seconds, err := strconv.ParseFloat(strings.TrimSuffix(parts[3], "\""), 64)
	if err != nil {
		return 0, err
	}

	direction := parts[4]

	decimal := degrees + minutes/60 + seconds/3600

	// Apply the direction to get the correct sign
	if direction == "S" || direction == "W" {
		decimal = -decimal
	}

	return decimal, nil
}

func extractMetadata(et *exiftool.Exiftool, imagePath string) (*ImageMetadata, error) {
	fileInfos := et.ExtractMetadata(imagePath)
	if len(fileInfos) == 0 {
		return nil, fmt.Errorf("no metadata found for image: %s", imagePath)
	}

	fileInfo := fileInfos[0]

	latStr, latOk := fileInfo.Fields["GPSLatitude"].(string)
	lonStr, lonOk := fileInfo.Fields["GPSLongitude"].(string)
	alt, altOk := fileInfo.Fields["GPSAltitude"].(string)
	dateTimeStr, dateTimeOk := fileInfo.Fields["GPSDateTime"].(string)

	if !latOk || !lonOk {
		fmt.Printf("Warning: Missing geolocation data for image: %s\n", imagePath)
	}

	if !altOk {
		fmt.Printf("Warning: Missing altitude data for image: %s\n", imagePath)
	}

	if !dateTimeOk {
		return nil, fmt.Errorf("could not extract necessary datetime metadata for image: %s", imagePath)
	}

	// Convert latitude and longitude to decimal degrees
	lat, err := convertDMSToDecimal(latStr)
	if err != nil {
		return nil, fmt.Errorf("failed to convert latitude: %v", err)
	}

	lon, err := convertDMSToDecimal(lonStr)
	if err != nil {
		return nil, fmt.Errorf("failed to convert longitude: %v", err)
	}

	// Parse the combined date and time string into a time.Time value
	dateTime, err := time.Parse("2006:01:02 15:04:05Z", dateTimeStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse combined date and time: %v", err)
	}

	return &ImageMetadata{
		Latitude:  lat,
		Longitude: lon,
		Altitude:  alt,
		DateTime:  dateTime,
	}, nil
}

func generateMarkdown(metadata *ImageMetadata, imagePath, outputDir string) error {
	// Create the directory based on the date
	// Create the markdown content
	mdContent := fmt.Sprintf(
		"---\nimage: \"%s\"\nlatitude: %f\nlongitude: %f\ndatetime: \"%s\"\n---\n",
		filepath.Base(imagePath),
		metadata.Latitude,
		metadata.Longitude,
		metadata.DateTime.Format(time.RFC3339),
	)

	// Write the markdown file
	mdFileName := strings.TrimSuffix(filepath.Base(imagePath), filepath.Ext(imagePath)) + ".md"
	mdFilePath := filepath.Join(outputDir, mdFileName)

	err := os.WriteFile(mdFilePath, []byte(mdContent), 0644)
	if err != nil {
		return err
	}

	return nil
}

// copyFile copies a file from src to dst. If dst already exists, it will be overwritten.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	err = out.Sync()
	if err != nil {
		return err
	}

	return nil
}

func main() {
	inputDir := "./content/images"
	outputDir := "./content/images"

	// Initialize Exiftool
	et, err := exiftool.NewExiftool()
	if err != nil {
		fmt.Printf("Error when intializing Exiftool: %v\n", err)
		return
	}
	defer et.Close()

	// Process images in the input directory
	err = filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && (strings.ToLower(filepath.Ext(path)) == ".jpg" || strings.ToLower(filepath.Ext(path)) == ".jpeg") {
			metadata, err := extractMetadata(et, path)
			if err != nil {
				fmt.Printf("Failed to process %s: %v\n", path, err)
				return nil
			}

			err = generateMarkdown(metadata, path, outputDir)
			if err != nil {
				fmt.Printf("Failed to generate markdown for %s: %v\n", path, err)
				return nil
			}

			fmt.Printf("Processed %s successfully.\n", path)
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking through input directory: %v\n", err)
	}
}
