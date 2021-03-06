package mysql

import (
	"database/sql"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/influxdb/telegraf/plugins"
)

type Mysql struct {
	Servers []string
}

var sampleConfig = `
  # specify servers via a url matching:
  #  [username[:password]@][protocol[(address)]]/[?tls=[true|false|skip-verify]]
  #  see https://github.com/go-sql-driver/mysql#dsn-data-source-name
  #  e.g.
  #    root:passwd@tcp(127.0.0.1:3306)/?tls=false
  #    root@tcp(127.0.0.1:3306)/?tls=false
  #
  # If no servers are specified, then localhost is used as the host.
  servers = ["tcp(127.0.0.1:3306)/"]
`

func (m *Mysql) SampleConfig() string {
	return sampleConfig
}

func (m *Mysql) Description() string {
	return "Read metrics from one or many mysql servers"
}

var localhost = ""

func (m *Mysql) Gather(acc plugins.Accumulator) error {
	if len(m.Servers) == 0 {
		// if we can't get stats in this case, thats fine, don't report
		// an error.
		m.gatherServer(localhost, acc)
		return nil
	}

	for _, serv := range m.Servers {
		err := m.gatherServer(serv, acc)
		if err != nil {
			return err
		}
	}

	return nil
}

type mapping struct {
	onServer string
	inExport string
}

var mappings = []*mapping{
	{
		onServer: "Aborted_",
		inExport: "aborted_",
	},
	{
		onServer: "Bytes_",
		inExport: "bytes_",
	},
	{
		onServer: "Com_",
		inExport: "commands_",
	},
	{
		onServer: "Created_",
		inExport: "created_",
	},
	{
		onServer: "Handler_",
		inExport: "handler_",
	},
	{
		onServer: "Innodb_",
		inExport: "innodb_",
	},
	{
		onServer: "Key_",
		inExport: "key_",
	},
	{
		onServer: "Open_",
		inExport: "open_",
	},
	{
		onServer: "Opened_",
		inExport: "opened_",
	},
	{
		onServer: "Qcache_",
		inExport: "qcache_",
	},
	{
		onServer: "Table_",
		inExport: "table_",
	},
	{
		onServer: "Tokudb_",
		inExport: "tokudb_",
	},
	{
		onServer: "Threads_",
		inExport: "threads_",
	},
}

func (m *Mysql) gatherServer(serv string, acc plugins.Accumulator) error {
	// If user forgot the '/', add it
	if strings.HasSuffix(serv, ")") {
		serv = serv + "/"
	} else if serv == "localhost" {
		serv = ""
	}

	db, err := sql.Open("mysql", serv)
	if err != nil {
		return err
	}

	defer db.Close()

	rows, err := db.Query(`SHOW /*!50002 GLOBAL */ STATUS`)
	if err != nil {
		return err
	}

	var servtag string
	servtag, err = parseDSN(serv)
	if err != nil {
		servtag = "localhost"
	}
	for rows.Next() {
		var name string
		var val interface{}

		err = rows.Scan(&name, &val)
		if err != nil {
			return err
		}

		var found bool

		tags := map[string]string{"server": servtag}

		for _, mapped := range mappings {
			if strings.HasPrefix(name, mapped.onServer) {
				i, _ := strconv.Atoi(string(val.([]byte)))
				acc.Add(mapped.inExport+name[len(mapped.onServer):], i, tags)
				found = true
			}
		}

		if found {
			continue
		}

		switch name {
		case "Queries":
			i, err := strconv.ParseInt(string(val.([]byte)), 10, 64)
			if err != nil {
				return err
			}

			acc.Add("queries", i, tags)
		case "Slow_queries":
			i, err := strconv.ParseInt(string(val.([]byte)), 10, 64)
			if err != nil {
				return err
			}

			acc.Add("slow_queries", i, tags)
		}
	}

	conn_rows, err := db.Query("SELECT user, sum(1) FROM INFORMATION_SCHEMA.PROCESSLIST GROUP BY user")

	for conn_rows.Next() {
		var user string
		var connections int64

		err = conn_rows.Scan(&user, &connections)
		if err != nil {
			return err
		}

		tags := map[string]string{"server": servtag, "user": user}

		if err != nil {
			return err
		}
		acc.Add("connections", connections, tags)
	}

	return nil
}

func init() {
	plugins.Add("mysql", func() plugins.Plugin {
		return &Mysql{}
	})
}
