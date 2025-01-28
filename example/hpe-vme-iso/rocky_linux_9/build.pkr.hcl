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
  boot_command = [
    "<up><tab> text ip={{ .StaticIP }}::{{ .StaticGateway }}:{{ .StaticMask }}:${local.vm_name}:ens3:none nameserver={{ .StaticDNS }} inst.ks=http://{{ .HTTPIP }}:{{ .HTTPPort }}/ks.cfg<enter><wait><enter>"
  ]
  vm_name   = "packiso-${local.timestamp}"
  static_ip = "{{ .StaticIP }}"
}

source "hpe-vme-iso" "demo" {
  url                 = var.vme_url
  username            = var.vme_username
  password            = var.vme_password
  cluster_name            = "vmecluster01"
  boot_command            = local.boot_command
  boot_wait               = "3s"
  http_interface          = "en0"
  http_directory          = "${path.root}/http"
  http_template_directory = "${path.root}/http_templates"
  http_port_min           = 8020
  http_port_max           = 8030
  convert_to_template     = true
  vm_name                 = local.vm_name
  template_name           = "rockytemplate"
  virtual_image_id        = 31
  group                   = "Platform Engineering"
  cloud                   = "HPE Demo"
  plan_id                 = 21
  ip_wait_timeout         = "25m"

  network_interface {
    network_id                = 3
    network_interface_type_id = 4
  }

  storage_volume {
    name            = "root"
    root_volume     = true
    size            = 25
    storage_type_id = 1
    datastore_id    = 2
  }

  # Raise the timeout, when installation takes longer
  ssh_timeout  = "55m"
  communicator = "ssh"
  ssh_username = "root"
  ssh_password = "mysecurepassword"
}

build {
  sources = [
    "source.hpe-vme-iso.demo"
  ]

  provisioner "shell" {
    script = "scripts/setup.sh"
  }
}
