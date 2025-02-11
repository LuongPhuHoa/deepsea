// Copyright © 2019 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"encoding/json"
	"os"
	"upper.io/db.v3"

	"upper.io/db.v3/ql"

	"deepsea/global"
	"github.com/spf13/cobra"
	jlog "github.com/spf13/jwalterweatherman"
	"github.com/spf13/viper"
)

var SourceFile string
var IdentifierRegex string
var DropTable bool

// loadCmd represents the load command
var loadCmd = &cobra.Command{
	Use:   "load",
	Short: "Load Marks from a file",
	Long:  `LOAD: Load Marks from a CSV file`,
	Run: func(cmd *cobra.Command, args []string) {
		jlog.DEBUG.Println("loadDriver()")
		loadDriver(cmd, args)
	},
}

func init() {

	loadCmd.Flags().StringVarP(&DBFile,
		"DBFile",
		"d",
		"",
		"Path to QL DB file")

	loadCmd.Flags().StringVarP(
		&SourceFile,
		"SourceFile",
		"s",
		"",
		"Path to Source of marks file")

	loadCmd.Flags().StringVarP(
		&IdentifierRegex,
		"IdentifierRegex",
		"r",
		"",
		"<dynamic> RegEx pattern")

	if err = viper.BindPFlag(
		"storage.DBFile",
		storageCmd.Flags().Lookup("DBFile")); err != nil {
		_ = storageCmd.Help()
		jlog.DEBUG.Println("Setting DBFile")
		os.Exit(2)
	}

	loadCmd.Flags().BoolVarP(&DropTable,
		"DropTable",
		"D",
		false,
		"Drop Table, do not truncate data")

	if err = viper.BindPFlag(
		"storage.load.SourceFile",
		loadCmd.Flags().Lookup("SourceFile")); err != nil {
		jlog.DEBUG.Println("Setting SourceFile")
		_ = loadCmd.Help()
		os.Exit(2)
	}
	if err = viper.BindPFlag(
		"storage.load.IdentifierRegex",
		loadCmd.Flags().Lookup("IdentifierRegex")); err != nil {
		jlog.DEBUG.Println("Setting IdentifierRegex")
		_ = loadCmd.Help()
		os.Exit(2)
	}

	storageCmd.AddCommand(loadCmd)
}

func loadDriver(cmd *cobra.Command, args []string) {

	var markCollection db.Collection

	var dbFile = viper.GetString("storage.DBFile")
	if ! global.FileExists(dbFile)  {
		jlog.ERROR.Fatalf("Database file does not exist: %s", dbFile )
	}

	var settings = ql.ConnectionURL{
		Database: viper.GetString("storage.DBFile"),
	}

	sess, err := ql.Open(settings)
	if err != nil {
		jlog.ERROR.Fatalf("db.Open(): %q\n", err)
	}
	defer sess.Close()

	markCollection = sess.Collection("mark")
	if !markCollection.Exists() {
		jlog.ERROR.Fatalf("Mark collection does not exist. Is mark schema loaded/table created?")
	}

	// Option A: Remove Mark table
	if DropTable {
		jlog.DEBUG.Printf("Dropping table Mark if exists\n")
		DropMarks(sess, markCollection)
		CreateMarks(sess, markCollection)

	}else{
		// Option B: Truncate Mark table data
		jlog.DEBUG.Printf("Selecting the mark table \n")
		TruncateMarks(sess,markCollection)
	}


	// Marks in CSV file
	if global.CSVFileRe.MatchString(
		viper.GetString("storage.load.SourceFile")) {

		jlog.DEBUG.Println("Matched Source File as a CSV file")
		var marks []global.Mark

		jlog.DEBUG.Println("Converting CSV2JSON for DB Load")
		marksJson, err := global.CSV2Json(
			viper.GetString("storage.load.SourceFile"))
		if err != nil {
			jlog.ERROR.Fatalf("Could not parse CSV Source File: %v", err)
		}
		jlog.TRACE.Println(string(marksJson))

		jlog.DEBUG.Println("Unmarshal JSON into Marks")
		err = json.Unmarshal(marksJson, &marks)
		if err != nil {
			jlog.ERROR.Fatalf("JSON Unmarshal error: %v", err)
		}

		jlog.DEBUG.Println("Loading Marks into DB")
		ix := 1
		for k := range marks {

			jlog.DEBUG.Printf("[%d] Loading Mark\n", ix)
			if len(marks[k].Email) == 0 {
				jlog.WARN.Println("	Mark has no email. Skip...")
				continue
			}

			if !global.EmailRe.MatchString(marks[k].Email) {
				jlog.WARN.Println("	Mark email format is invalid. Skip...")
				continue
			}

			if marks[k].Identifier == "<dynamic>" {
				jlog.TRACE.Println("Mark has dynamic ID")
				// Regex generate
				marks[k].Identifier, err = global.RegToString(
					viper.GetString("storage.load.IdentifierRegex"))
				if err != nil {
					// Fallback to random strings
					jlog.WARN.Println(
						"IdentifierRegex problem? Setting a random string id")
					marks[k].Identifier = global.RandString(8)
				}
			}

			// Inserting rows into the "Mark" table.
			jlog.DEBUG.Printf("Checks Passed. Inserting record.\n")
			_, err = markCollection.Insert(global.Mark{
				Identifier: marks[k].Identifier,
				Email:      marks[k].Email,
				Firstname:  marks[k].Firstname,
				Lastname:   marks[k].Lastname,
			})
		}
	}

	// Query for the results we've just inserted.
	jlog.TRACE.Println("Finding all marks")
	res := markCollection.Find()

	// Query all results and fill the mark variable with them.
	var marks []global.Mark

	jlog.TRACE.Println("Getting all marks from collection")
	err = res.All(&marks)
	if err != nil {
		jlog.ERROR.Printf("res.All(): %q\n", err)
		os.Exit(3)
	}

	jlog.INFO.Println("-= = = =  Marks = = = =-")
	ShowMarks(sess,markCollection)
}
