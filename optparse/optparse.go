// Public Domain (-) 2010-2013 The Golly Authors.
// See the Golly UNLICENSE file for details.

// Package optparse provides utility functions for the parsing and
// autocompletion of command line arguments.
package optparse

import (
	"fmt"
	"github.com/tav/golly/structure"
	"github.com/tav/golly/yaml"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Completer interface {
	Complete() []string
}

type listCompleter struct {
	items []string
}

func (completer *listCompleter) Complete() []string {
	return completer.items
}

func ListCompleter(items ...string) Completer {
	return &listCompleter{items}
}

func exit(message string, v ...interface{}) {
	if len(v) == 0 {
		fmt.Fprint(os.Stderr, message)
	} else {
		fmt.Fprintf(os.Stderr, message, v...)
	}
	os.Exit(1)
}

type Parser struct {
	Completer      Completer
	ParseHelp      bool
	ParseVersion   bool
	Usage          string
	Version        string
	nextCompleter  Completer
	nextDest       string
	nextRequired   bool
	options        []*option
	config2options map[string]*option
	configflags    []string
	short2options  map[string]*option
	shortflags     []string
	long2options   map[string]*option
	longflags      []string
	helpAdded      bool
	versionAdded   bool
}

type option struct {
	boolValue      *bool
	defined        bool
	dest           string
	completer      Completer
	configflag     string
	intValue       *int
	listValue      *[]string
	longflag       string
	requiredConfig bool
	requiredFlag   bool
	shortflag      string
	stringValue    *string
	descr          string
	valueType      string
}

func (opt *option) String() (output string) {
	output = "  "
	if opt.configflag != "" {
		output += opt.configflag
		output += ": "
	} else {
		if opt.shortflag != "" {
			output += opt.shortflag
			if opt.longflag != "" {
				output += ", "
			}
		}
		if opt.longflag != "" {
			output += opt.longflag
		}
		if opt.dest != "" {
			output += " " + opt.dest
		}
	}
	length := len(output)
	if length >= 21 {
		output += "\n                    "
	} else {
		padding := make([]byte, 20-length)
		for i, _ := range padding {
			padding[i] = 32
		}
		output += string(padding)
	}
	output += opt.descr
	output += "\n"
	return
}

func (op *Parser) computeFlags(flags []string, opt *option) (configflag, shortflag, longflag string) {
	for _, flag := range flags {
		if strings.HasPrefix(flag, "--") {
			longflag = flag
			op.long2options[longflag] = opt
			op.longflags = append(op.longflags, longflag)
		} else if strings.HasPrefix(flag, "-") {
			shortflag = flag
			op.short2options[shortflag] = opt
			op.shortflags = append(op.shortflags, shortflag)
		} else if strings.HasSuffix(flag, ":") {
			configflag = flag[0 : len(flag)-1]
			op.config2options[configflag] = opt
			op.configflags = append(op.configflags, configflag)
		} else {
			longflag = flag
			op.long2options[longflag] = opt
			op.longflags = append(op.longflags, longflag)
		}
	}
	return
}

func (op *Parser) newOpt(flags []string, descr string, displayDest bool) *option {
	opt := &option{}
	opt.descr = descr
	opt.configflag, opt.shortflag, opt.longflag = op.computeFlags(flags, opt)
	opt.completer = op.nextCompleter
	required := op.nextRequired
	if required {
		if opt.configflag == "" {
			opt.requiredFlag = true
		} else {
			opt.requiredConfig = true
		}
	}
	if displayDest {
		if op.nextDest != "" {
			opt.dest = op.nextDest
		} else {
			if opt.longflag != "" {
				opt.dest = strings.ToUpper(strings.TrimLeft(opt.longflag, "-"))
			} else {
				opt.dest = strings.ToUpper(strings.TrimLeft(opt.shortflag, "-"))
			}
		}
	}
	op.options = append(op.options, opt)
	op.nextCompleter = nil
	op.nextDest = ""
	op.nextRequired = false
	return opt
}

func (op *Parser) Int(flags []string, defaultValue int, descr string) *int {
	opt := op.newOpt(flags, descr, true)
	opt.valueType = "int"
	opt.intValue = &defaultValue
	return &defaultValue
}

func (op *Parser) String(flags []string, defaultValue string, descr string) *string {
	opt := op.newOpt(flags, descr, true)
	opt.valueType = "string"
	opt.stringValue = &defaultValue
	return &defaultValue
}

func (op *Parser) Bool(flags []string, descr string) *bool {
	defaultValue := false
	opt := op.newOpt(flags, descr, false)
	opt.valueType = "bool"
	opt.boolValue = &defaultValue
	return &defaultValue
}

func (op *Parser) IntConfig(key string, defaultValue int, descr string) *int {
	opt := op.newOpt([]string{key + ":", "--" + key}, descr, false)
	opt.valueType = "int"
	opt.intValue = &defaultValue
	return &defaultValue
}

func (op *Parser) StringConfig(key string, defaultValue string, descr string) *string {
	opt := op.newOpt([]string{key + ":", "--" + key}, descr, false)
	opt.valueType = "string"
	opt.stringValue = &defaultValue
	return &defaultValue
}

func (op *Parser) BoolConfig(key string, descr string) *bool {
	defaultValue := false
	opt := op.newOpt([]string{key + ":", "--" + key}, descr, false)
	opt.valueType = "bool"
	opt.boolValue = &defaultValue
	return &defaultValue
}

// Required indicates that the option parser should raise an
// error if the next defined option is not specified.
func (op *Parser) Required() *Parser {
	op.nextRequired = true
	return op
}

// WithOptCompleter will use the provided Completer to
// autocomplete the next defined option.
func (op *Parser) WithOptCompleter(c Completer) *Parser {
	op.nextCompleter = c
	return op
}

// As will use the given destination string for the next
// defined option.
func (op *Parser) As(destination string) *Parser {
	op.nextDest = destination
	return op
}

// Parse will parse the given args slice and try and define
// the defined options.
func (op *Parser) Parse(args []string) (remainder []string) {

	if op.ParseHelp && !op.helpAdded {
		op.Bool([]string{"-h", "--help"}, "show this help and exit")
		op.helpAdded = true
	}
	if op.ParseVersion && !op.versionAdded {
		op.Bool([]string{"-v", "--version"}, "show the version and exit")
		op.versionAdded = true
	}

	argLength := len(args) - 1
	complete, words, compWord, prefix := getCompletionData()

	// Command-line auto-completion support.
	if complete {

		// Pass to the shell completion if the previous word was a flag
		// expecting some parameter.
		if (compWord - 1) > 0 {
			var completer Completer
			prev := words[compWord-1]
			if strings.HasPrefix(prev, "--") {
				opt, ok := op.long2options[prev]
				if ok {
					if opt.dest != "" {
						if opt.completer == nil {
							os.Exit(1)
						} else {
							completer = opt.completer
						}
					}
				}
			} else if strings.HasPrefix(prev, "-") {
				opt, ok := op.short2options[prev]
				if ok {
					if opt.dest != "" {
						if opt.completer == nil {
							os.Exit(1)
						} else {
							completer = opt.completer
						}
					}
				}
			}
			if completer != nil {
				completions := make([]string, 0)
				for _, item := range completer.Complete() {
					if strings.HasPrefix(item, prefix) {
						completions = append(completions, item)
					}
				}
				fmt.Print(strings.Join(completions, " "))
				os.Exit(1)
			}
		}

		completions := make([]string, 0)

		if op.Completer != nil {
			for _, item := range op.Completer.Complete() {
				if strings.HasPrefix(item, prefix) {
					completions = append(completions, item)
				}
			}
		}

		for flag, _ := range op.long2options {
			if strings.HasPrefix(flag, prefix) {
				completions = append(completions, flag)
			}
		}

		for flag, _ := range op.short2options {
			if strings.HasPrefix(flag, prefix) {
				completions = append(completions, flag)
			}
		}

		fmt.Print(strings.Join(completions, " "))
		os.Exit(1)

	}

	if argLength == 0 {
		return
	}

	var opt *option
	var ok bool

	idx := 1

	for {
		arg := args[idx]
		noOpt := true
		if strings.HasPrefix(arg, "--") {
			opt, ok = op.long2options[arg]
			if ok {
				noOpt = false
			}
		} else if strings.HasPrefix(arg, "-") {
			opt, ok = op.short2options[arg]
			if ok {
				noOpt = false
			}
		} else {
			remainder = append(remainder, arg)
			if idx == argLength {
				break
			} else {
				idx += 1
				continue
			}
		}
		if noOpt {
			exit("%s: error: no such option: %s\n", args[0], arg)
		}
		if opt.dest != "" {
			if idx == argLength {
				exit("%s: error: %s option requires an argument\n", args[0], arg)
			}
		}
		if opt.valueType == "bool" {
			if opt.longflag == "--help" && op.ParseHelp {
				op.PrintUsage()
				os.Exit(1)
			} else if opt.longflag == "--version" && op.ParseVersion {
				fmt.Printf("%s\n", op.Version)
				os.Exit(0)
			}
			*opt.boolValue = true
			opt.defined = true
			idx += 1
		} else if opt.valueType == "string" {
			if idx == argLength {
				exit("%s: error: no value specified for %s\n", args[0], arg)
			}
			*opt.stringValue = args[idx+1]
			opt.defined = true
			idx += 2
		} else if opt.valueType == "int" {
			if idx == argLength {
				exit("%s: error: no value specified for %s\n", args[0], arg)
			}
			intValue, err := strconv.Atoi(args[idx+1])
			if err != nil {
				exit("%s: error: couldn't convert %s value '%s' to an integer\n", args[0], arg, args[idx+1])
			}
			*opt.intValue = intValue
			opt.defined = true
			idx += 2
		}
		if idx > argLength {
			break
		}
	}

	for _, opt := range op.options {
		if opt.requiredFlag && !opt.defined {
			exit("%s: error: required: %s", args[0], opt)
		}
	}

	return

}

func (op *Parser) ParseConfig(filename string, args []string) (err error) {

	data, err := yaml.ParseDictFile(filename)
	if err != nil {
		return err
	}

	for config, opt := range op.config2options {
		if opt.defined {
			continue
		}
		value, ok := data[config]
		if !ok {
			if opt.requiredConfig {
				exit("%s: error: required: %s", args[0], opt)
			} else {
				continue
			}
		}
		if opt.valueType == "bool" {
			if value == "true" || value == "on" || value == "yes" {
				*opt.boolValue = true
			} else if value == "false" || value == "off" || value == "no" {
				*opt.boolValue = false
			} else {
				exit("%s: error: invalid boolean value for %s: %q\n", args[0], config, value)
			}
		} else if opt.valueType == "string" {
			*opt.stringValue = value
		} else if opt.valueType == "int" {
			intValue, err := strconv.Atoi(value)
			if err != nil {
				exit("%s: error: couldn't convert the %s value %q to an integer\n", args[0], config, value)
			}
			*opt.intValue = intValue
		}
	}

	return nil

}

func (op *Parser) PrintUsage() {
	fmt.Print(op.Usage)
	if len(op.configflags) > 0 {
		fmt.Print("\nConfig File Options:\n")
	}
	for _, opt := range op.options {
		if opt.configflag != "" {
			fmt.Printf("%v", opt)
		}
	}
	if len(op.options) > 0 {
		fmt.Print("\nOptions:\n")
	}
	for _, opt := range op.options {
		if opt.configflag == "" {
			fmt.Printf("%v", opt)
		}
	}
}

func (op *Parser) PrintDefaultConfigFile(name string) {
	fmt.Printf("# %s.yaml\n\n", name)
	for _, opt := range op.options {
		if opt.configflag != "" {
			fmt.Printf("%s: ", opt.configflag)
			switch opt.valueType {
			case "int":
				fmt.Printf("%d\n", *opt.intValue)
			case "bool":
				fmt.Printf("%v\n", *opt.boolValue)
			case "string":
				fmt.Printf("%s\n", *opt.stringValue)
			}
		}
	}
}

// New takes the header and version for the usage string and
// returns a fresh option parser.
func New(usage string, version ...string) *Parser {
	op := &Parser{}
	op.long2options = make(map[string]*option)
	op.short2options = make(map[string]*option)
	op.config2options = make(map[string]*option)
	op.Usage = usage
	op.ParseHelp = true
	verSlice := []string(version)
	if len(verSlice) > 0 {
		op.ParseVersion = true
		op.Version = verSlice[0]
	} else {
		op.ParseVersion = false
	}
	return op
}

func getCompletionData() (complete bool, words []string, compWord int, prefix string) {

	autocomplete := os.Getenv("OPTPARSE_AUTO_COMPLETE")
	if autocomplete != "" {

		complete = true
		compWords := os.Getenv("COMP_WORDS")
		if compWords == "" {
			// zsh's bashcompinit does not pass COMP_WORDS, replace with
			// COMP_LINE for now...
			compWords = os.Getenv("COMP_LINE")
			if compWords == "" {
				os.Exit(1)
			}
		}

		words = strings.Split(compWords, " ")
		compLine := os.Getenv("COMP_LINE")

		compPoint, err := strconv.Atoi(os.Getenv("COMP_POINT"))
		if err != nil {
			os.Exit(1)
		}

		compWord, err = strconv.Atoi(os.Getenv("COMP_CWORD"))
		if err != nil {
			os.Exit(1)
		}

		if compWord > 0 {
			if compWord < len(words) {
				prefix = words[compWord]
			}
		}

		// At some point in the future, make use of these variables.
		_ = compLine
		_ = compPoint

	}

	return

}

// SubCommands provides support for git subcommands style command handling.
func SubCommands(name, version string, commands map[string]func([]string, string), commandsUsage map[string]string, additional ...string) {

	var commandNames, helpCommands []string
	var complete bool
	var mainUsage string

	callCommand := func(command string, args []string, complete bool) {
		var findexe bool
		if command[0] == '-' {
			args[0] = name
		} else {
			args[0] = fmt.Sprintf("%s %s", name, command)
			findexe = true
		}
		if handler, ok := commands[command]; ok {
			handler(args, commandsUsage[command])
		} else if findexe {

			exe := fmt.Sprintf("%s-%s", strings.Replace(name, " ", "-", -1), command)
			exePath, err := exec.LookPath(exe)
			if err != nil {
				exit("ERROR: Couldn't find '%s'\n", exe)
			}

			args[0] = exe
			process, err := os.StartProcess(exePath, args,
				&os.ProcAttr{
					Dir:   ".",
					Env:   os.Environ(),
					Files: []*os.File{nil, os.Stdout, os.Stderr},
				})

			if err != nil {
				exit(fmt.Sprintf("ERROR: %s: %s\n", exe, err))
			}

			_, err = process.Wait()
			if err != nil {
				exit(fmt.Sprintf("ERROR: %s: %s\n", exe, err))
			}

		} else {
			exit(fmt.Sprintf("%s: error: no such option: %s\n", name, command))
		}
		os.Exit(0)
	}

	if _, ok := commands["help"]; !ok {
		commands["help"] = func(args []string, usage string) {

			opts := New(mainUsage)
			opts.ParseHelp = false
			opts.Completer = ListCompleter(helpCommands...)
			helpArgs := opts.Parse(args)

			if len(helpArgs) == 0 {
				fmt.Print(mainUsage)
				return
			}

			if len(helpArgs) != 1 {
				exit("ERROR: Unknown command '%s'\n", strings.Join(helpArgs, " "))
			}

			command := helpArgs[0]
			if command == "help" {
				fmt.Print(mainUsage)
			} else {
				if !complete {
					argLen := len(os.Args)
					os.Args[argLen-2], os.Args[argLen-1] = os.Args[argLen-1], "--help"
				}
				callCommand(command, []string{name, "--help"}, false)
			}

		}
		commands["-h"] = commands["help"]
		commands["--help"] = commands["help"]
	}

	if len(version) != 0 {
		if _, ok := commands["version"]; !ok {
			commands["version"] = func(args []string, usage string) {
				opts := New(fmt.Sprintf("Usage: %s version\n\n    %s\n", name, usage))
				opts.Parse(args)
				fmt.Printf("%s\n", version)
				return
			}
			commands["-v"] = commands["version"]
			commands["--version"] = commands["version"]
		}
	}

	commandNames = make([]string, len(commands))
	helpCommands = make([]string, len(commands))
	i, j := 0, 0

	for name, _ := range commands {
		if !strings.HasPrefix(name, "-") {
			commandNames[i] = name
			i += 1
			if name != "help" {
				helpCommands[j] = name
				j += 1
			}
		}
	}

	usageKeys := structure.SortedKeys(commandsUsage)
	padding := 10

	for _, key := range usageKeys {
		if len(key) > padding {
			padding = len(key)
		}
	}

	var suffix string

	additionalItems := len(additional)
	if additionalItems == 0 {
		suffix = ""
	} else if additionalItems == 1 {
		mainUsage = additional[0] + "\n"
		suffix = ""
	} else {
		mainUsage = additional[0] + "\n"
		suffix = "\n" + additional[1]
	}

	mainUsage += fmt.Sprintf("Usage: %s <command> [options]\n\nCommands:\n\n", name)
	usageLine := fmt.Sprintf("    %%-%ds %%s\n", padding)

	for _, key := range usageKeys {
		mainUsage += fmt.Sprintf(usageLine, key, commandsUsage[key])
	}

	mainUsage += suffix
	mainUsage += fmt.Sprintf(
		"\nSee `%s help <command>` for more info on a specific command.\n", name)

	complete, words, compWord, prefix := getCompletionData()
	baseLength := len(strings.Split(name, " "))
	args := os.Args

	if complete && len(args) == 1 {
		if compWord == baseLength {
			completions := make([]string, 0)
			for _, cmd := range commandNames {
				if strings.HasPrefix(cmd, prefix) {
					completions = append(completions, cmd)
				}
			}
			fmt.Print(strings.Join(completions, " "))
			os.Exit(1)
		} else {
			command := words[baseLength]
			args = []string{name}
			callCommand(command, args, true)
		}
	}

	args = args[baseLength:]

	if len(args) == 0 {
		fmt.Print(mainUsage)
		os.Exit(0)
	}

	command := args[0]
	args[0] = name

	callCommand(command, args, false)

}
