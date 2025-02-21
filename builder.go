package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cmd184psu/alfredo"
)

type argStruct struct {
	Publish bool // upload rpm or binary to public repo
	//upload the rpm to a public s3 bucket, github release (optional)

	Install bool // upload and install rpm
	//upload most recent RPM to select machines and install the rpm over ssh
	//or copy the binary to /usr/local/bin (or /usr/bin)

	Build bool // build binary or rpm
	//build the binary or RPM

	// SSHKey string
	// // local path to ssh key (stored in build.json or ~/build.json)

	// SSHIP string
	// // IP address to set for use with ssh; store in build.json

	// SSHUser string
	// // user to use for ssh; store in build.json

	// BuildRemoteDir string
	// // build directory to use on remote machine over ssh

	Show bool
	//parse and show configuration

	SelfCheck bool
	//check if ssh is configured correctly / remote machien is reachable

	SkipBuild bool
}

func parseArgs() *argStruct {
	args := new(argStruct)
	//var workercsv string
	//MISC
	var verbose bool
	var panic bool
	var debug bool
	var experimental bool
	var ver bool
	flag.BoolVar(&verbose, "verbose", false, "verbose mode")
	flag.BoolVar(&ver, "ver", false, "show version")
	flag.BoolVar(&ver, "version", false, "show version")
	flag.BoolVar(&panic, "panic", false, "panic on error")
	flag.BoolVar(&debug, "debug", false, "debug mode")
	flag.BoolVar(&experimental, "experimental", false, "experimental mode")
	flag.BoolVar(&experimental, "exp", false, "experimental mode")

	flag.BoolVar(&args.Publish, "pub", false, "publish latest RPM package")
	flag.BoolVar(&args.Publish, "publish", false, "publish latest RPM package")
	flag.BoolVar(&args.Install, "install", false, "install latest RPM package, on each remote system")
	flag.BoolVar(&args.Build, "build", false, "build RPM package or local binary")
	flag.BoolVar(&args.SkipBuild, "skipbuild", false, "skip build during installation")

	// flag.StringVar(&args.SSHKey,"sshkey", "", "Path to ssh key")
	// flag.StringVar(&args.SSHIP,"sship", "", "IP of remote system")
	// flag.StringVar(&args.SSHUser,"sshuser", "", "user on remote system")
	// flag.StringVar(&args.BuildRemoteDir,"remotedir", "", "remote directory to use for build")
	flag.BoolVar(&args.Show, "show", false, "show configuration")
	flag.BoolVar(&args.SelfCheck, "check", false, "Check to see that connectivity to remote systems is operational.")

	flag.Parse()

	if verbose {
		alfredo.SetVerbose(verbose)
	}
	if panic {
		alfredo.SetPanic(panic)
	}
	if debug {
		alfredo.SetDebug(debug)
	}
	if experimental {
		alfredo.SetExperimental(experimental)
	}

	if ver {
		fmt.Printf(builder_version_fmt, BuildVersion())
		os.Exit(0)
	}

	return args
}

type systemStruct struct {
	Ssh      alfredo.SSHStruct `json:"ssh"`
	Name     string            `json:"name"`
	Rpm      bool              `json:"rpm"`
	RpmArch  string            `json:"rpmarch"`
	Filename string            `json:"filename"`
}

type configStruct struct {
	BuildSystem    systemStruct   `json:"buildSystem"`
	InstallTargets []systemStruct `json:"installTargets"`
	BuildCli       string         `json:"buildCLI"`   //build binary
	PublishCli     string         `json:"publishCLI"` //build RPM?
	PackageName    string         `json:"packageName"`
}

func (config *configStruct) Load(filename string) error {
	alfredo.VerbosePrintln("BEGIN Load(" + filename + ")")
	if alfredo.FileExistsEasy(filename) {
		if err := alfredo.ReadStructFromJSONFile(filename, &config); err != nil {
			panic("unable to parse configuration file: " + filename + ", use jq to check format")
			//			alfredo.VerbosePrintln("issue loading config (1)::" + err.Error())
			//			return err
		}
	} else {
		jsonContent := "{}"
		if err := json.Unmarshal([]byte(jsonContent), &config); err != nil {
			alfredo.VerbosePrintln("issue loading config (2)::" + err.Error())
			return err
		}
	}
	return nil
}

func GetRPMFilenameFromFiles() string {
	if !alfredo.FileExistsEasy("./RPMNAME") {
		panic("Missing RPMNAME file, can't copy")
	}
	if !alfredo.FileExistsEasy("./VERSION") {
		panic("Missing VERSION file, can't copy")
	}
	if !alfredo.FileExistsEasy("./RELEASE") {
		panic("Missing RELEASE file, can't copy")
	}

	rpm := GetFirstLineFromFile("./RPMNAME")
	ver := GetFirstLineFromFile("./VERSION")
	var rel string
	if alfredo.FileExistsEasy("./RELEASE") {
		rel = GetFirstLineFromFile("./RELEASE")
	} else {
		rel = "1"
	}
	var arch string
	if alfredo.FileExistsEasy("./ARCH") {
		arch = GetFirstLineFromFile("./ARCH")
	} else {
		arch = "x86_64"
	}

	return fmt.Sprintf("%s-%s-%s.%s.rpm", rpm, ver, rel, arch)
}

//lint:ignore ST1006 no reason
func (this *configStruct) Save(filename string) error {
	alfredo.VerbosePrintln("config.save()::outputile is " + filename)
	err := alfredo.WriteStructToJSONFilePP(filename, this)
	return err
}

const default_local_ssh_key = "./ssh_access_key"
const default_local_ssh_user = "root"

func (config configStruct) Show() configStruct {
	fmt.Println("Build System:")
	fmt.Printf("\tName:%s\n", config.BuildSystem.Name)
	fmt.Printf("\tHost:%s\n", config.BuildSystem.Ssh.Host)
	fmt.Printf("\tSSH Key:%s\n", config.BuildSystem.Ssh.Key)
	fmt.Printf("\tUser:%s\n", config.BuildSystem.Ssh.User)
	fmt.Printf("\tRemote Dir:%s\n", config.BuildSystem.Ssh.GetRemoteDir())

	fmt.Println("Install Targets:")
	for i := 0; i < len(config.InstallTargets); i++ {
		fmt.Printf("\tName:%s\n", config.InstallTargets[i].Name)
		fmt.Printf("\tHost:%s\n", config.InstallTargets[i].Ssh.Host)
		fmt.Printf("\tSSH Key:%s\n", config.InstallTargets[i].Ssh.Key)
		fmt.Printf("\tUser:%s\n", config.InstallTargets[i].Ssh.User)
		fmt.Printf("\tRemote Dir:%s\n", config.InstallTargets[i].Ssh.GetRemoteDir())
	}

	if runtime.GOOS == "darwin" {
		fmt.Println("\tBuild SSH CLI: " + config.BuildSystem.Ssh.GetSSHCli() + " \"make clean rpms\"")
	}
	return config
}

// constants
const default_config_path = "./build.json"
const rpm_top = "/opt/rpmbuild"

func GetRPMFileName() string {
	// currentUser, err := user.Current()
	// if err != nil {
	// 	panic("Unable to know current user")
	// }

	//return filepath.Join(currentUser.HomeDir, "rpmbuild/RPMS/x86_64/"+GetRPMFilenameFromFiles())
	return filepath.Join(rpm_top, "/RPMS/x86_64/"+GetRPMFilenameFromFiles())
}

func GetFirstLineFromFile(f string) string {
	if !alfredo.FileExistsEasy(f) {
		panic("File " + f + " does not exist or is not readable")
	}
	file, err := os.Open(f)
	if err != nil {
		panic("Error reading file " + f)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		return scanner.Text()
	}
	panic("Error reading file " + f)
}

// func RefreshHarness(config configStruct) {
// 	var sshErr error
// 	sshErr = config.Source.SecureUploadAndSpin(default_config_path, default_config_path)
// 	if sshErr != nil {
// 		panic(sshErr.Error())
// 	}
// 	sshErr = config.Source.SecureUploadAndSpin("./testharness-linux", "/usr/bin/testharness")
// 	if sshErr != nil {
// 		panic(sshErr.Error())
// 	}
// 	sshErr = config.Source.SecureRemoteExecution("chmod 755 /usr/bin/testharness")
// 	if sshErr != nil {
// 		panic(sshErr.Error())
// 	}
// }

func (config configStruct) SelfCheck() error {
	alfredo.VerbosePrintln("BEGIN: config.SelfCheck()")
	fmt.Println("Build System:")
	fmt.Printf("\t")
	if err := config.BuildSystem.Ssh.SecureRemoteExecution("ls -lah"); err != nil {
		return alfredo.PanicError(err.Error())
	}
	fmt.Println(config.BuildSystem.Ssh.GetBody())
	//fmt.Printf("status: %d\n",config.BuildSystem.Ssh.)
	fmt.Println("Install Targets:")
	fmt.Printf("\t")
	for i := 0; i < len(config.InstallTargets); i++ {
		if err := config.InstallTargets[i].Ssh.SecureRemoteExecution("hostname -s"); err != nil {
			return alfredo.PanicError(err.Error())
		}
	}
	alfredo.VerbosePrintln("END: config.SelfCheck()")
	return nil
}

func BuildVersion() string {

	alfredo.VerbosePrintln("gitbranch=" + GitBranch)
	alfredo.VerbosePrintln("ver=" + GitVersion)
	alfredo.VerbosePrintln("time=" + GitTimestamp)

	var gb string
	if strings.EqualFold(GitBranch, "main") {
		gb = ""
	} else {
		gb = "-" + GitBranch
	}

	return fmt.Sprintf("%s%s (%s)", GitVersion, gb, GitTimestamp)
}

// # APPNAME := bucket-migrator
// # PACKAGE := github.com/cloudian/bucket-migrator/version
// # REVISION := $(shell git rev-parse --short HEAD)
// # BRANCH := $(shell git rev-parse --abbrev-ref HEAD | tr -d '\040\011\012\015\n')
// # TIME := $(shell date +%d%_b%_y-%I:%M%_\#p)

// THDIR=cbm-testharness

// R=$(cd $THDIR && git rev-parse --short HEAD)
// B=$(cd $THDIR && git rev-parse --abbrev-ref HEAD | tr -d '\040\011\012\015\n')
// GV=$(cd $THDIR && cat ./VERSION)
// TS=$(date +%d%_b%_y-%I:%M%p)
// export LF="-X main.GitRevision=\"$R\" -X main.GitBranch=\"$B\" -X main.GitVersion=\"$GV\" -X main.GitTimestamp=\"$TS\""

// if [[ -e /etc/redhat-release ]]; then
//    EXE=$(cat /etc/redhat-release | grep 6)
// fi

// if [[ -z "$EXE" ]]; then
//         echo "build the old way"
//         (cd $THDIR  && GOOS=linux GOARCH=amd64  go build -ldflags "$LF" -o ../testharness-linux)
// echo " (cd $THDIR  && GOOS=linux GOARCH=amd64  go build -ldflags "$LF" -o ../testharness-linux)"

//         A=$(arch)
//         U=$(uname)
//         if [[ "$U" == "Darwin" ]]; then
// #               if [[ "$A" == "i386" ]]; then
//                    (cd $THDIR  && GOOS=darwin GOARCH=amd64 go build -ldflags "$LF" -o ../testharness-mac-amd64)
// #               else
//                    (cd $THDIR  && GOOS=darwin GOARCH=arm64 go build -ldflags "$LF" -o ../testharness-mac-arm64)
// #               fi
//                 lipo -create -output testharness-mac testharness-mac-amd64 testharness-mac-arm64
//        fi
// else
//         (cd $THDIR  && GOOS=linux GOARCH=amd64  go build -ldflags "$LF" -o ../testharness-linux6)
// fi

const builder_version_fmt = "Builder (c) C Delezenski <cmd184psu@gmail.com> - %s"

func (config *configStruct) FixSSHKeys() {
	if len(config.BuildSystem.Ssh.Key) == 0 {
		config.BuildSystem.Ssh.Key = alfredo.SSH_DEFAULT_KEY
	}
}

func (config *configStruct) ReInstallRPM() error {
	ver := config.BuildSystem.Ssh.RemoteGetVersion()
	// if err := config.BuildSystem.Ssh.RemoteReadFile("VERSION"); err != nil {
	// 	return alfredo.PanicError(err.Error())
	// }
	// ver := strings.Trim(config.BuildSystem.Ssh.GetBody(), "\n")
	fmt.Printf("version=%s\n", ver)
	// if err := config.BuildSystem.Ssh.RemoteReadFile("RELEASE"); err != nil {
	// 	return alfredo.PanicError(err.Error())
	// }
	// fmt.Printf("release(?)=%q\n", config.BuildSystem.Ssh.GetBody())
	// rel, _ := strconv.Atoi(strings.Trim(config.BuildSystem.Ssh.GetBody(), "\n"))
	rel := config.BuildSystem.Ssh.RemoteGetRelease()
	fmt.Printf("release=%d\n", rel)
	suffix := fmt.Sprintf("-%s-%d.%s.rpm", ver, rel, config.BuildSystem.RpmArch)
	fmt.Printf("suffix=%s\n", suffix)
	prefix := alfredo.GetBaseName(config.BuildSystem.Filename)
	prefix = prefix[:len(prefix)-len(suffix)]
	var cli []string
	var msg []string
	//msg = append(msg, "")
	//		var temp string
	for i := 0; i < len(config.InstallTargets); i++ {
		cli = nil
		msg = nil

		fmt.Printf("Uploading %s to %s\n", alfredo.GetBaseName(config.BuildSystem.Filename), config.InstallTargets[i].Ssh.Host)
		config.BuildSystem.Ssh.CrossCopy(config.BuildSystem.Filename, config.InstallTargets[i].Ssh, "~/"+alfredo.GetBaseName(config.BuildSystem.Filename))

		msg = append(msg, fmt.Sprintf("Removing prior install on %s\n", config.InstallTargets[i].Ssh.Host))
		cli = append(cli, fmt.Sprintf("rpm -e %s || /bin/true", prefix))

		msg = append(msg, fmt.Sprintf("Installing new copy on %s\n", config.InstallTargets[i].Ssh.Host))
		cli = append(cli, fmt.Sprintf("rpm -iUvh %s%s", config.InstallTargets[i].Ssh.RemoteDir,
			alfredo.GetBaseName(config.BuildSystem.Filename)))

		for j := 0; j < len(msg); j++ {
			fmt.Println(msg[j])
			alfredo.VerbosePrintf("%s %q", config.InstallTargets[i].Ssh.GetSSHCli(), cli[j])

			if err := config.InstallTargets[i].Ssh.RemoteExecuteAndSpin(cli[j]); err != nil {
				fmt.Printf("exit code = %d\n", config.InstallTargets[i].Ssh.GetExitCode())
				fmt.Println(config.InstallTargets[i].Ssh.GetStderr())
				alfredo.VerbosePrintln("operation failed")
				return alfredo.PanicError(err.Error())
			} else {
				fmt.Println(config.InstallTargets[i].Ssh.GetStdout())
			}
		}
	}
	return nil
}

// func (config *configStruct) BuildGoBinary() error {
// 	alfredo.VerbosePrintln("BEGIN BuildGoBinary()")
// 	cli := config.BuildSystem.Ssh.GenerateGoBuildCLI(config.BuildSystem.PackageName)
// 	if err := config.BuildSystem.Ssh.RemoteExecuteAndSpin(cli); err != nil {
// 		return err
// 	}
// 	alfredo.VerbosePrintln("END BuildGoBinary()")
// 	return nil
// }

func (config *configStruct) ReInstallBinary() error {
	alfredo.VerbosePrintln("BEGIN ReInstallBinary()")
	var cli []string
	var msg []string
	for i := 0; i < len(config.InstallTargets); i++ {
		cli = nil
		msg = nil

		fmt.Printf("Uploading %s to %s\n", alfredo.GetBaseName(config.BuildSystem.Filename), config.InstallTargets[i].Ssh.Host)
		alfredo.VerbosePrintln(config.BuildSystem.Ssh.CrossCopyCLI(config.BuildSystem.Filename, config.InstallTargets[i].Ssh, config.InstallTargets[i].Ssh.GetRemoteDir()+"/"+alfredo.GetBaseName(config.BuildSystem.Filename)))
		if err := config.BuildSystem.Ssh.CrossCopy(config.BuildSystem.Filename, config.InstallTargets[i].Ssh, config.InstallTargets[i].Ssh.GetRemoteDir()+"/"+alfredo.GetBaseName(config.BuildSystem.Filename)); err != nil {
			return alfredo.PanicError(err.Error())
		}

		// msg = append(msg, fmt.Sprintf("Removing prior install on %s\n", config.InstallTargets[i].Ssh.Host))
		// cli = append(cli, fmt.Sprintf("rpm -e %s || /bin/true", prefix))

		msg = append(msg, fmt.Sprintf("Version check on %s\n", config.InstallTargets[i].Ssh.Host))
		cli = append(cli, fmt.Sprintf("%s -ver", config.InstallTargets[i].Ssh.GetRemoteDir()+"/"+config.PackageName))

		for j := 0; j < len(msg); j++ {
			fmt.Println(msg[j])
			alfredo.VerbosePrintf("%s %q", config.InstallTargets[i].Ssh.GetSSHCli(), cli[j])

			if err := config.InstallTargets[i].Ssh.RemoteExecuteAndSpin(cli[j]); err != nil {
				fmt.Printf("exit code = %d\n", config.InstallTargets[i].Ssh.GetExitCode())
				fmt.Println(config.InstallTargets[i].Ssh.GetStderr())
				alfredo.VerbosePrintln("operation failed")
				return alfredo.PanicError(err.Error())
			} else {
				fmt.Println(config.InstallTargets[i].Ssh.GetStdout())
			}
		}
	}

	alfredo.VerbosePrintln("END ReInstallBinary()")
	return nil
}

func main() {
	args := parseArgs()

	alfredo.VerbosePrintln("alternative boo")
	alfredo.VerbosePrintln(alfredo.ExpandTilde("~boo/some/other/stuff.txt"))
	alfredo.VerbosePrintln("in my home directory:")
	alfredo.VerbosePrintln(alfredo.ExpandTilde("~/some/other/stuff.txt"))

	var config configStruct

	if alfredo.FileExistsEasy(default_config_path) {
		alfredo.VerbosePrintln("file exists: " + default_config_path)

		config.Load(default_config_path)

	} else {
		alfredo.VerbosePrintln("no config to load")
	}

	config.FixSSHKeys()

	if args.SelfCheck {
		if err := config.SelfCheck(); err != nil {
			panic(err.Error())
		}
	}
	if args.Build || (args.Install && !args.SkipBuild) {
		if !config.BuildSystem.Rpm {
			config.BuildCli = config.BuildSystem.Ssh.GenerateRemoteGoBuildCLI(config.PackageName)
		}
		if len(config.BuildCli) == 0 {
			fmt.Println("missing build cli")
			os.Exit(1)
		}
		alfredo.VerbosePrintf("%s %q", config.BuildSystem.Ssh.GetSSHCli(), fmt.Sprintf("cd %s; %s", config.BuildSystem.Ssh.GetRemoteDir(), config.BuildCli))
		fmt.Printf("Building...%s...", config.PackageName)

		if err := config.BuildSystem.Ssh.RemoteExecuteAndSpin(config.BuildCli); err != nil {
			fmt.Println(config.BuildSystem.Ssh.GetStderr())
			panic(err.Error())
		}

		if config.BuildSystem.Rpm {
			temp := alfredo.GetFirstLineFromSlice(config.BuildSystem.Ssh.GetBody(), "Wrote")
			config.BuildSystem.Filename = strings.Replace(temp[7:], "/root", "/opt", 1)
			fmt.Printf("Wrote: %s\n", config.BuildSystem.Filename)
		} else {
			config.BuildSystem.Filename = config.BuildSystem.Ssh.GetRemoteDir() + "/" + config.PackageName
		}

		if alfredo.GetVerbose() {
			alfredo.VerbosePrintln(config.BuildSystem.Ssh.GetBody())
		}
		fmt.Println("build complete")

	}

	if args.Install {
		fmt.Printf("About to install on %d target systems\n", len(config.InstallTargets))
		//"filename": "%s/%s-%s-%d.%s.rpm"

		if config.BuildSystem.Rpm {
			if err := config.ReInstallRPM(); err != nil {
				panic(err.Error())
			}
		} else {
			if err := config.ReInstallBinary(); err != nil {
				panic(err.Error())
			}
		}

		fmt.Println("Installation Complete")

	}
	// 	var buildErr error
	// 	if runtime.GOOS == "darwin" {
	// 		VerbosePrintln("Mac detected, build over ssh")
	// 		VerbosePrintln("remotedir is: " + config.BuildDir)
	// 		config.Build.SetRemoteDir(config.BuildDir)
	// 		fmt.Println("remote dir: " + config.Build.GetRemoteDir())
	// 		buildErr = config.Build.RemoteExecuteAndSpin("go build")
	// 		if buildErr != nil {
	// 			panic(buildErr.Error())
	// 		}

	// 	} else {
	// 		fmt.Println("running a local build")
	// 		buildErr = localExec("go build")
	// 		if buildErr != nil {
	// 			panic(buildErr.Error())
	// 		}
	// 		buildErr = localExec(build_cli)
	// 		if buildErr != nil {
	// 			panic(buildErr.Error())
	// 		}

	// 		rpmfilename := GetRPMFileName()
	// 		destRPMpath := GetRPMFilenameFromFiles()

	// 		//copy RPM locally
	// 		_, copyErr := alfredo.CopyFile(rpmfilename, destRPMpath)
	// 		if copyErr != nil {
	// 			panic("Error copying from " + rpmfilename + " to ./" + destRPMpath)
	// 		}
	// 	}
	// 	if !args.NoUpload {
	// 		destRPMpath := GetRPMFilenameFromFiles()

	// 		if uploaderr := UploadAndInstallRPM(config, destRPMpath); uploaderr != nil {
	// 			panic(uploaderr.Error())
	// 		}
	// 	}
	// }

	if args.Show {
		config.Show()
	}
	config.Save(default_config_path)
	alfredo.VerbosePrintln("complete")
}
