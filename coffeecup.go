package main

import (
	"fmt"
	"time"

	mcli "github.com/jxskiss/mcli"
	"github.com/ttacon/chalk"
)

func main() {
	mcli.Add("login", LoginCommand, "Login to CoffeeCup")
	mcli.Add("start", StartCommand, "Start/Resume time entry")
	mcli.Add("today", TodayCommand, "Show today's time entries")

	mcli.Add("projects list", ProjectsListCommand, "Lists all projects")
	mcli.Add("projects alias", ProjectAliasCommand, "Lists the known aliases or sets new ones")

	// Enable shell auto-completion, see `program completion -h` for help.
	mcli.AddCompletion()

	mcli.Run()
}

func LoginCommand() {
	var args struct {
		CompanyUrl string `cli:"#R, -c, --company, The prefix of the company's CoffeeCup instance (the \"amazon\" in \"amazon.coffeecup.app\")"`
		Username   string `cli:"#R, -u, --username, The username of the user"`
		Password   string `cli:"#R, -p, --password, The password of the user"`
	}
	_, err := mcli.Parse(&args)
	if err != nil {
		panic(err)
	}

	accessToken, refreshToken, err := LoginWithPassword(args.CompanyUrl, args.Username, args.Password)
	if err != nil {
		panic(err)
	}

	StoreTokens(accessToken, refreshToken)

	userId, err := GetUserId()
	if err != nil {
		panic(err)
	}

	StoreUserId(userId)
	fmt.Printf("Successfully logged in as %s\n", args.Username)
}

func LoginUsingRefreshToken() {
	accessToken, refreshToken, err := LoginWithRefreshToken()
	if err != nil {
		panic(err)
	}

	StoreTokens(accessToken, refreshToken)
}

func ProjectsListCommand() {
	projects, err := GetProjects()

	// retry if unauthorized
	if err != nil && err.Error() == "unauthorized" {
		LoginUsingRefreshToken()
		projects, err = GetProjects()
	}

	if err != nil {
		panic(err)
	}

	for _, project := range projects {
		fmt.Printf("%d: %s\n", project.Id, project.Name)
	}
}

func ProjectAliasCommand() {
	var args struct {
		ProjectId int    `cli:"id, The ID of the project"`
		Alias     string `cli:"alias, The alias of the project"`
	}
	_, err := mcli.Parse(&args)
	if err != nil {
		panic(err)
	}

	cfg := ReadConfig()
	if cfg.Projects.Aliases == nil {
		cfg.Projects.Aliases = make(map[string]int)
	}

	if (args.ProjectId != 0) && (args.Alias == "") {
		fmt.Println("Please provide an alias for the project")
		return
	} else if (args.ProjectId == 0) && (args.Alias == "") {
		fmt.Println("Configured aliases:")
		for alias, projectId := range cfg.Projects.Aliases {
			fmt.Printf("%s: %d\n", alias, projectId)
		}
		return
	} else {
		cfg.Projects.Aliases[args.Alias] = args.ProjectId
		WriteConfig(cfg)
	}
}

func StartCommand() {
	var args struct {
		Alias   string `cli:"#R, alias, The alias of the project"`
		Comment string `cli:"comment, The comment for the time entry"`
	}
	_, err := mcli.Parse(&args)
	if err != nil {
		panic(err)
	}

	timeEntries, err := GetTodaysTimeEntries()
	// retry if unauthorized
	if err != nil && err.Error() == "unauthorized" {
		LoginUsingRefreshToken()
		timeEntries, err = GetTodaysTimeEntries()
	}

	if err != nil {
		panic(err)
	}

	projectAliases := ReadConfig().Projects.Aliases
	resumedExistingTimeEntry := false
	for _, timeEntry := range timeEntries {
		if projectAliases[args.Alias] == timeEntry.ProjectId {
			if timeEntry.Running {
				fmt.Printf("%s%s%s is running already\n", chalk.Green, args.Alias, chalk.Reset)
				if args.Comment != "" {
					if timeEntry.Comment == "" {
						timeEntry.Comment = "- " + args.Comment
					} else {
						timeEntry.Comment = timeEntry.Comment + "\n- " + args.Comment
					}
					fmt.Printf("Added comment to %s%s%s\n", chalk.Green, args.Alias, chalk.Reset)
					UpdateTimeEntry(timeEntry)
				}
				return
			}

			// not running, resume it
			timeEntry.Running = true
			UpdateTimeEntry(timeEntry)
			fmt.Printf("Resumed previous time entry for %s%s%s\n", chalk.Green, args.Alias, chalk.Reset)
			resumedExistingTimeEntry = true
		} else {
			if timeEntry.Running {
				timeEntry.Running = false
				var projectAlias string
				for alias, projectId := range projectAliases {
					if projectId == timeEntry.ProjectId {
						projectAlias = alias
						break
					}
				}
				fmt.Printf("Stopped %s%s%s\n", chalk.Red, projectAlias, chalk.Reset)
				UpdateTimeEntry(timeEntry)
			}
		}
	}

	if !resumedExistingTimeEntry {
		// start a new time entry
		projectId := projectAliases[args.Alias]
		today := time.Now().Format("2013-07-21")
		err := CreateTimeEntry(NewTimeEntry{
			ProjectId: projectId,
			Day:       today,
			Duration:  0,
			Sorting:   len(timeEntries) + 1,
			Running:   true,
			Comment:   "- " + args.Comment,
			// hardcoded task id for "Frontend" for now
			TaskId:       1095,
			TrackingType: "WORK",
			// hardcoded team id for "Allianz" for now
			TeamId: 402,
			UserId: GetUserIdFromConfig(),
		})
		if err != nil {
			panic(err)
		}

		fmt.Printf("Started new time entry for %s%s%s\n", chalk.Green, args.Alias, chalk.Reset)
	}
}

func TodayCommand() {
	timeEntries, err := GetTodaysTimeEntries()

	// retry if unauthorized
	if err != nil && err.Error() == "unauthorized" {
		LoginUsingRefreshToken()
		timeEntries, err = GetTodaysTimeEntries()
	}

	if err != nil {
		panic(err)
	}

	cfg := ReadConfig()
	aliases := cfg.Projects.Aliases

	if timeEntries == nil {
		fmt.Println("No time entries for today")
		return
	}

	for _, timeEntry := range timeEntries {
		hours := timeEntry.Duration / 3600
		minutes := (timeEntry.Duration % 3600) / 60

		var projectAlias string
		for alias, projectId := range aliases {
			if projectId == timeEntry.ProjectId {
				projectAlias = alias
				break
			}
		}

		// todo: use more colors with chalk
		fmt.Printf("Project: %s\nDuration: %d:%d\nComment:\n%s\n\n", projectAlias, hours, minutes, timeEntry.Comment)
	}
}
