package main

// Vendor drivers register themselves from init (database/sql style), the
// same set as cmd/bmcproxy/drivers.go.
import (
	_ "github.com/slothnetworks/bmcproxy/vendors/asrock"          // ASRock Rack (AMI MegaRAC /api + Redfish + IPMI)
	_ "github.com/slothnetworks/bmcproxy/vendors/generic/redfish" // generic DMTF Redfish fallback
	_ "github.com/slothnetworks/bmcproxy/vendors/gigabyte"        // Gigabyte / Giga Computing (Redfish-first, AMI MegaRAC /api lazily)
)
