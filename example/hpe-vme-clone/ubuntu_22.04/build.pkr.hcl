/*
packer {
  required_plugins {
    hpe-vme = {
      version = "0.0.1"
      source  = "github.com/martezr/hpe-vme"
    }
  }
}
*/

locals {
 timestamp = formatdate("mmss", timestamp())
}

source "hpe-vme-clone" "demo" {
  url = var.morpheus_url
  username = var.morpheus_username
  password = var.morpheus_password
  cluster_name = "vmecluster01"
  convert_to_template = true
  skip_agent_install = true
  vm_name = "pack-${local.timestamp}"
  template_name = "packertest"
  virtual_image_id = 986
  group_id = 3
  cloud_id = 139
  plan_id = 174

  network_interface {
    network_id = 229
    network_interface_type_id = 4
  }

  storage_volume {
    name = "root"
    root_volume = true
    size = 40
    storage_type_id = 1
    datastore_id = 49
  }
  communicator          = "none"
}

build {
  sources = [
    "source.hpe-vme-clone.demo"
  ]

  provisioner "mvm-morpheus" {
    url = var.morpheus_url
    username = var.morpheus_username
    password = var.morpheus_password
    task_id = 63
  }
}
