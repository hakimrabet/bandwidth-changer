package main

import (
	"flag"
	"fmt"
	"golang.org/x/crypto/ssh"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

func readBuffForString(whattoexpect string, sshOut io.Reader, buffRead chan<- string) {
	buf := make([]byte, 1000)
	n, err := sshOut.Read(buf) //this reads the ssh terminal
	waitingString := ""
	if err == nil {
		waitingString = string(buf[:n])
	}
	for (err == nil) && (!strings.Contains(waitingString, whattoexpect)) {
		n, err = sshOut.Read(buf)
		waitingString += string(buf[:n])
		//fmt.Println(waitingString)
	}
	buffRead <- waitingString
}
func readBuff(whattoexpect string, sshOut io.Reader, timeoutSeconds int) string {
	ch := make(chan string)
	go func(whattoexpect string, sshOut io.Reader) {
		buffRead := make(chan string)
		go readBuffForString(whattoexpect, sshOut, buffRead)
		select {
		case ret := <-buffRead:
			ch <- ret
		case <-time.After(time.Duration(timeoutSeconds) * time.Second):
			handleError(fmt.Errorf("%d", timeoutSeconds), true, "Waiting for \""+whattoexpect+"\" took longer than %s seconds, perhaps you've entered incorrect details?")
		}
	}(whattoexpect, sshOut)
	return <-ch
}
func writeBuff(command string, sshIn io.WriteCloser) (int, error) {
	returnCode, err := sshIn.Write([]byte(command + "\r"))
	return returnCode, err
}
func handleError(e error, fatal bool, customMessage ...string) {
	var errorMessage string
	if e != nil {
		if len(customMessage) > 0 {
			errorMessage = strings.Join(customMessage, " ")
		} else {
			errorMessage = "%s"
		}
		if fatal == true {
			log.Fatalf(errorMessage, e)
		} else {
			log.Print(errorMessage, e)
		}
	}
}
func main() {
	//argsWithoutProg := os.Args[1:]
	if len(os.Args) == 2 {
		if os.Args[1] == "-h" {
			fmt.Println("Version 0.1")
			fmt.Println("This program build by hakimrabet for changing bandwidth of interfaces")
			fmt.Println("Use it like below\n", os.Args[0], "ip_address", "username", "password", "enable_password", "interface", "Bandwidth")
		}
		return
	}
	if len(os.Args) != 7 {
		fmt.Println("for help run with -h")
		fmt.Println("Usage:", os.Args[0], "ip_address", "username", "password", "enable_password", "interface", "Bandwidth")
		return
	}
	var (
		ipAddress       string = os.Args[1]
		username        string = os.Args[2]
		password        string = os.Args[3]
		enable_password string = os.Args[4]
		interf          string = os.Args[5]
		bandwidth       string = os.Args[6]
	)
	var ip = flag.String("ip", ipAddress, "location of the switch to manage")
	var userName = flag.String("userName", username, "username to connect to switch")
	var normalPw = flag.String("normalPW", password, "the standard switch ssh password")
	var enablePw = flag.String("enablePW", enable_password, "the enable password for esculated priv")
	var interface_cisco = flag.String("interface", interf, "specify interface")
	var interface_bandwidth = flag.String("bandwidth", bandwidth, "size of bandwidth")
	/*
	   	var ip = flag.String("ip", "192.168.1.3", "location of the switch to manage")
	   	var userName = flag.String("userName", "kayer", "username to connect to switch")
	   	var normalPw = flag.String("normalPW", "kayer", "the standard switch ssh password")
	   	var enablePw = flag.String("enablePW", "kayer", "the enable password for esculated priv")
	   	var interface_cisco = flag.String("interface", "gig1/0/47", "specify interface")
	   	var interface_bandwidth = flag.String("interface", "50000", "size of bandwidth")
	   //	var tftpServer = flag.String("tftpServer", "192.168.1.66", "the tftp server ip address")
	*/
	flag.Parse()
	//fmt.Println("IP Chosen: ", *ip)
	//fmt.Println("Username", *userName)
	//fmt.Println("Normal PW", *normalPw)
	//fmt.Println("Enable PW", *enablePw)
	//fmt.Println("Interface", *interface_cisco)
	//fmt.Println("BandWidth", *interface_bandwidth)
	sshConfig := &ssh.ClientConfig{
		User: *userName,
		Auth: []ssh.AuthMethod{
			ssh.Password(*normalPw),
		},
		HostKeyCallback: func(ipAddress string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}
	sshConfig.Config.Ciphers = append(sshConfig.Config.Ciphers, "aes128-cbc")
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // disable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}
	connection, err := ssh.Dial("tcp", *ip+":22", sshConfig)
	if err != nil {
		log.Fatalf("Failed to dial: %s", err)
	}
	session, err := connection.NewSession()
	handleError(err, true, "Failed to create session: %s")

	sshOut, err := session.StdoutPipe()
	handleError(err, true, "Unable to setup stdin for session: %v")
	sshIn, err := session.StdinPipe()
	handleError(err, true, "Unable to setup stdout for session: %v")
	if err := session.RequestPty("xterm", 0, 200, modes); err != nil {
		session.Close()
		handleError(err, true, "request for pseudo terminal failed: %s")
	}
	if err := session.Shell(); err != nil {
		session.Close()
		handleError(err, true, "request for shell failed: %s")
	}
	readBuff(">", sshOut, 2)
	if _, err := writeBuff("enable", sshIn); err != nil {
		handleError(err, true, "Failed to run: %s")
	}
	if _, err := writeBuff(*enablePw, sshIn); err != nil {
		handleError(err, true, "Failed to run: %s")
	}
	readBuff("#", sshOut, 2)
	if _, err := writeBuff("conf t", sshIn); err != nil {
		handleError(err, true, "Failed to run: %s")
	}
	readBuff("#", sshOut, 2)
	if _, err := writeBuff("int  "+*interface_cisco, sshIn); err != nil {
		handleError(err, true, "Failed to run: %s")
	}
	readBuff(")#", sshOut, 2)
	if _, err := writeBuff("bandwidth "+*interface_bandwidth, sshIn); err != nil {
		handleError(err, true, "Failed to run: %s")
	}
	var out = readBuff("#", sshOut, 60)
	if !strings.Contains(out, "'^' marker") {
		fmt.Println("Bandwidth of " + *interface_cisco + " changed to" + *interface_bandwidth)
	} else {
		fmt.Println("Error occured")
	}
	session.Close()
}
