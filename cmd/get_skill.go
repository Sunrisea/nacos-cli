package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nov11/nacos-cli/internal/client"
	"github.com/nov11/nacos-cli/internal/help"
	"github.com/nov11/nacos-cli/internal/skill"
	"github.com/spf13/cobra"
)

var (
	getSkillOutput string
)

var getSkillCmd = &cobra.Command{
	Use:   "skill-get [skillName]",
	Short: "Get a skill and download it locally",
	Long:  help.SkillGet.FormatForCLI("nacos-cli"),
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		skillName := args[0]

		// Default output directory
		if getSkillOutput == "" {
			homeDir, err := os.UserHomeDir()
			checkError(err)
			getSkillOutput = filepath.Join(homeDir, ".skills")
		} else {
			// Expand ~ to home directory
			if strings.HasPrefix(getSkillOutput, "~/") {
				homeDir, err := os.UserHomeDir()
				checkError(err)
				getSkillOutput = filepath.Join(homeDir, getSkillOutput[2:])
			} else if getSkillOutput == "~" {
				homeDir, err := os.UserHomeDir()
				checkError(err)
				getSkillOutput = homeDir
			}
		}

		// Create Nacos client
		nacosClient := client.NewNacosClient(serverAddr, namespace, authType, username, password, accessKey, secretKey)

		// Create skill service
		skillService := skill.NewSkillService(nacosClient)

		// Get skill
		fmt.Printf("Fetching skill: %s...\n", skillName)
		err := skillService.GetSkill(skillName, getSkillOutput)
		checkError(err)

		skillPath := filepath.Join(getSkillOutput, skillName)
		fmt.Printf("Skill downloaded successfully!\n")
		fmt.Printf("  Location: %s\n", skillPath)
	},
}

func init() {
	getSkillCmd.Flags().StringVarP(&getSkillOutput, "output", "o", "", "Output directory (default: ~/.skills)")
	rootCmd.AddCommand(getSkillCmd)
}
