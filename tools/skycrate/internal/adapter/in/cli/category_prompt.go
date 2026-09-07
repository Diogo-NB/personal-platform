package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/category"
)

func selectCategory(
	ctx context.Context,
	input io.Reader,
	output io.Writer,
	catalog *category.Catalog,
) (string, error) {
	categories := catalog.Categories()
	reader := bufio.NewReader(input)

	if _, err := fmt.Fprintln(output, "Select a category:"); err != nil {
		return "", fmt.Errorf("write category picker: %w", err)
	}
	for index, categoryPath := range categories {
		if _, err := fmt.Fprintf(output, "  %d. %s\n", index+1, categoryPath); err != nil {
			return "", fmt.Errorf("write category picker: %w", err)
		}
	}

	selected, err := readCategorySelection(ctx, reader, output, categories)
	if err != nil {
		return "", err
	}

	return readCategorySuffix(ctx, reader, output, catalog, selected)
}

func readCategorySelection(
	ctx context.Context,
	reader *bufio.Reader,
	output io.Writer,
	categories []string,
) (string, error) {
	for {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("select category: %w", err)
		}
		if _, err := fmt.Fprint(output, "Selection: "); err != nil {
			return "", fmt.Errorf("write category prompt: %w", err)
		}

		line, err := readLine(reader)
		if err != nil {
			return "", fmt.Errorf("read category selection: %w", err)
		}
		selection, conversionErr := strconv.Atoi(strings.TrimSpace(line))
		if conversionErr == nil && selection >= 1 && selection <= len(categories) {
			return categories[selection-1], nil
		}

		if _, err := fmt.Fprintf(
			output,
			"Invalid selection. Enter a number from 1 to %d.\n",
			len(categories),
		); err != nil {
			return "", fmt.Errorf("write category validation: %w", err)
		}
	}
}

func readCategorySuffix(
	ctx context.Context,
	reader *bufio.Reader,
	output io.Writer,
	catalog *category.Catalog,
	selected string,
) (string, error) {
	for {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("select category suffix: %w", err)
		}
		if _, err := fmt.Fprint(output, "Subcategory (optional): "); err != nil {
			return "", fmt.Errorf("write subcategory prompt: %w", err)
		}

		line, err := readLine(reader)
		if err != nil {
			return "", fmt.Errorf("read subcategory: %w", err)
		}
		suffix := strings.TrimSpace(line)
		if suffix == "" {
			return selected, nil
		}

		normalized, err := category.NormalizePath(suffix)
		if err == nil {
			combined := selected + "/" + normalized
			resolution, resolutionErr := catalog.Resolve(combined)
			if resolutionErr == nil {
				return resolution.Category, nil
			}
			err = resolutionErr
		}

		if _, writeErr := fmt.Fprintf(output, "Invalid subcategory: %v\n", err); writeErr != nil {
			return "", fmt.Errorf("write subcategory validation: %w", writeErr)
		}
	}
}

func readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err == nil {
		return line, nil
	}
	if errors.Is(err, io.EOF) && line != "" {
		return line, nil
	}
	return "", err
}
