package cmd

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"bufio"

	"github.com/coniks-sys/coniks-go/client"
	"github.com/coniks-sys/coniks-go/keyserver/testutil"
	p "github.com/coniks-sys/coniks-go/protocol"
	"github.com/spf13/cobra"
)

const help = "- register [name] [key]:\r\n" +
	"	Register a new name-to-key binding on the CONIKS-server.\r\n" +
	"- lookup [name]:\r\n" +
	"	Lookup the key of some known contact or your own bindings.\r\n" +
	"- enable timestamp:\r\n" +
	"	Print timestamp of format <15:04:05.999999999> along with the result.\r\n" +
	"- disable timestamp:\r\n" +
	"	Disable timestamp printing.\r\n" +
	"- help:\r\n" +
	"	Display this message.\r\n" +
	"- exit, q:\r\n" +
	"	Close the REPL and exit the client."

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the test client.",
	Long:  "Run gives you a REPL, so that you can invoke commands to perform CONIKS operations including registration and key lookup. Currently, it supports:\n" + help,
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd)
	},
}

func init() {
	RootCmd.AddCommand(runCmd)
	runCmd.Flags().StringP("config", "c", "config.toml",
		"Config file for the client (contains the server's initial public key etc).")
	runCmd.Flags().BoolP("debug", "d", false, "Turn on debugging mode")
}

func handleQueryConnection(cc *p.ConsistencyChecks, conf *client.Config, conn net.Conn) {
	fmt.Println("Handling new connection...")

	defer func() {
		fmt.Println("Closing connection...")
		conn.Close()
	}()

	bufReader := bufio.NewReader(conn)
	bytes, err := bufReader.ReadBytes('\n')
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("received string: %s", bytes)

	keystr := strings.TrimSpace(fmt.Sprintf("%s", bytes))

	msg := keyLookup(cc, conf, keystr)
	conn.Write([]byte(msg))
}

func run(cmd *cobra.Command) {
	conf := loadConfigOrExit(cmd)
	cc := p.NewCC(nil, true, conf.SigningPubKey)

	listener, err:= net.Listen("tcp", ":6601")
	if err != nil {
		fmt.Println(err)
		return
	}

	defer func() {
		listener.Close()
		fmt.Println("Listener closed")
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println(err)
			break
		}

		go handleQueryConnection(cc, conf, conn)
	}
}

func keyLookup(cc *p.ConsistencyChecks, conf *client.Config, name string) string {
	req, err := client.CreateKeyLookupMsg(name)
	if err != nil {
		return ("")
	}

	var res []byte
	u, _ := url.Parse(conf.Address)
	switch u.Scheme {
	case "tcp":
		res, err = testutil.NewTCPClient(req, conf.Address)
		if err != nil {
			return ("")
		}
	case "unix":
		res, err = testutil.NewUnixClient(req, conf.Address)
		if err != nil {
			return ("")
		}
	default:
		return ("")
	}

	response := client.UnmarshalResponse(p.KeyLookupType, res)
	if key, ok := cc.Bindings[name]; ok {
		err = cc.HandleResponse(p.KeyLookupType, response, name, []byte(key))
	} else {
		err = cc.HandleResponse(p.KeyLookupType, response, name, nil)
	}

	switch err {
	case p.CheckBadSTR:
		return ("")
	case p.CheckPassed:
		switch response.Error {
		case p.ReqSuccess:
			key, err := response.GetKey()
			if err != nil {
				return ("")
			}
			return (string(key))
		case p.ReqNameNotFound:
			return ("")
		}
	default:
		return ("")
	}
	return ("")
}
