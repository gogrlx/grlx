package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/gogrlx/grlx/v2/internal/api/client"
	"github.com/gogrlx/grlx/v2/internal/log"
)

// RecipeInfo mirrors the natsapi.RecipeInfo type for CLI unmarshaling.
type RecipeInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// RecipeContent mirrors the natsapi.RecipeContent type for CLI unmarshaling.
type RecipeContent struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int64  `json:"size"`
}

var cmdRecipes = &cobra.Command{
	Use:   "recipes",
	Short: "Manage and inspect recipes",
}

var cmdRecipesList = &cobra.Command{
	Use:   "list",
	Short: "List available recipes on the farmer",
	Run: func(cmd *cobra.Command, args []string) {
		result, err := client.NatsRequest("recipes.list", nil)
		if err != nil {
			log.Fatal(err)
		}

		switch outputMode {
		case "json":
			fmt.Println(string(result))
		default:
			var resp struct {
				Recipes []RecipeInfo `json:"recipes"`
			}
			if err := json.Unmarshal(result, &resp); err != nil {
				log.Fatal(err)
			}
			if len(resp.Recipes) == 0 {
				fmt.Println("No recipes found.")
				return
			}
			printRecipesTable(resp.Recipes)
		}
	},
}

var cmdRecipesShow = &cobra.Command{
	Use:   "show <recipe-name>",
	Short: "Show the content of a recipe",
	Long: `Show the full content of a recipe file.
Recipe names use dot notation (e.g., "webserver.nginx").`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		params := map[string]string{"name": args[0]}
		result, err := client.NatsRequest("recipes.get", params)
		if err != nil {
			log.Fatal(err)
		}

		switch outputMode {
		case "json":
			fmt.Println(string(result))
		default:
			var recipe RecipeContent
			if err := json.Unmarshal(result, &recipe); err != nil {
				log.Fatal(err)
			}

			header := color.New(color.FgCyan, color.Bold)
			header.Printf("Recipe: %s\n", recipe.Name)
			fmt.Printf("Path:   %s\n", recipe.Path)
			fmt.Printf("Size:   %d bytes\n", recipe.Size)
			fmt.Println(strings.Repeat("─", 60))
			fmt.Println(recipe.Content)
		}
	},
}

func printRecipesTable(recipes []RecipeInfo) {
	header := color.New(color.FgCyan, color.Bold)

	// Find max name width for alignment
	nameWidth := len("RECIPE")
	pathWidth := len("PATH")
	for _, r := range recipes {
		if len(r.Name) > nameWidth {
			nameWidth = len(r.Name)
		}
		if len(r.Path) > pathWidth {
			pathWidth = len(r.Path)
		}
	}

	fmtStr := fmt.Sprintf("%%-%ds  %%-%ds  %%s\n", nameWidth, pathWidth)
	header.Printf(fmtStr, "RECIPE", "PATH", "SIZE")

	for _, r := range recipes {
		sizeStr := formatSize(r.Size)
		fmt.Printf(fmtStr, r.Name, r.Path, sizeStr)
	}

	fmt.Printf("\n%d recipe(s)\n", len(recipes))
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMG"[exp])
}

func init() {
	cmdRecipes.AddCommand(cmdRecipesList)
	cmdRecipes.AddCommand(cmdRecipesShow)
	rootCmd.AddCommand(cmdRecipes)
}
