package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func ConnectTovCenter(ctx context.Context, hostname, username, password string, insecure bool) *govmomi.Client {
	u, err := url.Parse(hostname)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	u.User = url.UserPassword(username, password)
	c, err := govmomi.NewClient(ctx, u, insecure)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	return c
}

func GetAllVMs(ctx context.Context, c *govmomi.Client) []mo.VirtualMachine {

	// set datacenter
	f := find.NewFinder(c.Client, true)
	dc, err := f.DefaultDatacenter(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	f.SetDatacenter(dc)

	// get vm list
	vms, err := f.VirtualMachineList(ctx, "*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	// convert to MOB Reference
	var refs []types.ManagedObjectReference
	for _, vm := range vms {
		refs = append(refs, vm.Reference())
	}

	// GetAll Properties
	pc := property.DefaultCollector(c.Client)
	var vmt []mo.VirtualMachine
	err = pc.Retrieve(ctx, refs, nil, &vmt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	return vmt
}

func GetAllHosts(ctx context.Context, c *govmomi.Client) []mo.HostSystem {
	// set datacenter
	f := find.NewFinder(c.Client, true)
	dc, err := f.DefaultDatacenter(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	f.SetDatacenter(dc)

	// get vm list
	hosts, err := f.HostSystemList(ctx, "*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	// convert to MOB Reference
	var refs []types.ManagedObjectReference
	for _, host := range hosts {
		refs = append(refs, host.Reference())
	}

	// GetAll Properties
	pc := property.DefaultCollector(c.Client)
	var hst []mo.HostSystem
	err = pc.Retrieve(ctx, refs, nil, &hst)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	return hst
}

func SyncVMRecord(ctx context.Context, etcdEndpoint string, rootPath string, etcdDomainName string, vms []mo.VirtualMachine) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{etcdEndpoint},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close() // make sure to close the client
	for _, vm := range vms {
		if len(vm.Summary.Guest.IpAddress) != 0 { //put record if vm has ipaddress. vCLS is excluded.
			_, err = cli.Put(context.TODO(), rootPath+etcdDomainName+"/"+strings.Split(vm.Config.Name, ".")[0], "{\"host\":\""+vm.Summary.Guest.IpAddress+"\",\"ttl\":60}")
			if err != nil {
				log.Fatal(err)
			}
		} else { //delete record if vm does not have ipaddress.
			_, err = cli.Delete(context.TODO(), rootPath+etcdDomainName+"/"+strings.Split(vm.Config.Name, ".")[0])
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func SyncHostRecord(ctx context.Context, etcdEndpoint string, rootPath string, etcdDomainName string, hst []mo.HostSystem) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{etcdEndpoint},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close() // make sure to close the client
	for _, hs := range hst {
		if len(hs.Summary.ManagementServerIp) != 0 { //put record if host has ipaddress. vCLS is excluded.
			_, err = cli.Put(context.TODO(), rootPath+etcdDomainName+"/"+strings.Split(hs.Summary.Config.Name, ".")[0], "{\"host\":\""+hs.Summary.ManagementServerIp+"\",\"ttl\":60}")
			if err != nil {
				log.Fatal(err)
			}
		} else { //delete record if host does not have ipaddress.
			_, err = cli.Delete(context.TODO(), rootPath+etcdDomainName+"/"+strings.Split(hs.Summary.Config.Name, ".")[0])
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := ConnectTovCenter(ctx, os.Getenv("vCSAHostname"), os.Getenv("vCSAUserName"), os.Getenv("vCSAPassword"), true)
	vms := GetAllVMs(ctx, c)
	SyncVMRecord(ctx, os.Getenv("etcdEndpoint"), os.Getenv("etcdPluginRootPath"), os.Getenv("etcdDomainName"), vms)
	hosts := GetAllHosts(ctx, c)
	SyncHostRecord(ctx, os.Getenv("etcdEndpoint"), os.Getenv("etcdPluginRootPath"), os.Getenv("etcdDomainName"), hosts)
}

