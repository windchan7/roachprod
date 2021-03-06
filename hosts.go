package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/cockroachdb/roachprod/cloud"
	"github.com/cockroachdb/roachprod/config"
	"github.com/cockroachdb/roachprod/install"
	"github.com/pkg/errors"
)

const (
	sshKeyPathPrefix = "${HOME}/.ssh/roachprod"
)

func initHostDir() error {
	hd := os.ExpandEnv(config.DefaultHostDir)
	return os.MkdirAll(hd, 0755)
}

func syncHosts(cloud *cloud.Cloud) error {
	hd := os.ExpandEnv(config.DefaultHostDir)

	// Write all host files.
	for _, c := range cloud.Clusters {
		filename := path.Join(hd, c.Name)
		file, err := os.Create(filename)
		if err != nil {
			return errors.Wrapf(err, "problem creating file %s", filename)
		}
		defer file.Close()

		// Align columns left and separate with at least two spaces.
		tw := tabwriter.NewWriter(file, 0, 8, 2, ' ', 0)
		tw.Write([]byte("# user@host\tlocality\tvpcId\n"))
		for _, vm := range c.VMs {
			tw.Write([]byte(fmt.Sprintf(
				"%s@%s\t%s\t%s\n", vm.RemoteUser, vm.PublicIP, vm.Locality(), vm.VPC)))
		}
		if err := tw.Flush(); err != nil {
			return errors.Wrapf(err, "problem writing file %s", filename)
		}
	}

	return gcHostsFiles(cloud)
}

func gcHostsFiles(cloud *cloud.Cloud) error {
	hd := os.ExpandEnv(config.DefaultHostDir)
	files, err := ioutil.ReadDir(hd)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.Mode().IsRegular() {
			continue
		}
		if _, ok := cloud.Clusters[file.Name()]; ok {
			continue
		}

		filename := filepath.Join(hd, file.Name())
		if err = os.Remove(filename); err != nil {
			log.Printf("failed to remove file %s", filename)
		}
	}
	return nil
}

func newInvalidHostsLineErr(line string) error {
	return fmt.Errorf("invalid hosts line, expected <username>@<host> [locality] [vpcId], got %q", line)
}

func loadClusters() error {
	hd := os.ExpandEnv(config.DefaultHostDir)
	files, err := ioutil.ReadDir(hd)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.Mode().IsRegular() {
			continue
		}

		filename := filepath.Join(hd, file.Name())
		contents, err := ioutil.ReadFile(filename)
		if err != nil {
			return errors.Wrapf(err, "could not read %s", filename)
		}
		lines := strings.Split(string(contents), "\n")

		c := &install.SyncedCluster{
			Name: file.Name(),
		}

		for _, l := range lines {
			// We'll consume the fields as we go along
			fields := strings.Fields(l)
			if len(fields) == 0 {
				continue
			} else if len(fields[0]) > 0 && fields[0][0] == '#' {
				// Comment line.
				continue
			} else if len(fields) > 3 {
				return newInvalidHostsLineErr(l)
			}

			parts := strings.Split(fields[0], "@")
			fields = fields[1:]
			var n, u string
			if len(parts) == 1 {
				u = config.OSUser.Username
				n = parts[0]
			} else if len(parts) == 2 {
				u = parts[0]
				n = parts[1]
			} else {
				return newInvalidHostsLineErr(l)
			}

			var locality string
			if len(fields) > 0 {
				locality = fields[0]
				fields = fields[1:]
			}

			var vpc string
			if len(fields) > 0 {
				vpc = fields[0]
				fields = fields[1:]
			}

			if len(fields) > 0 {
				return newInvalidHostsLineErr(l)
			}

			c.VMs = append(c.VMs, n)
			c.Users = append(c.Users, u)
			c.Localities = append(c.Localities, locality)
			c.VPCs = append(c.VPCs, vpc)
		}
		install.Clusters[file.Name()] = c
	}

	return nil
}
