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
  timestamp        = formatdate("mmss", timestamp())
  template_version = formatdate("MM_DD_YYYY", timestamp())
  patch_cycle      = formatdate("MMM_YYYY", timestamp())
  boot_command = [
    "<up><tab> text ip={{ .StaticIP }}::{{ .StaticGateway }}:{{ .StaticMask }}:${local.vm_name}:ens3:none nameserver={{ .StaticDNS }} inst.ks=http://{{ .HTTPIP }}:{{ .HTTPPort }}/ks.cfg<enter><wait><enter>"
  ]
  vm_name   = "packiso-${local.timestamp}"
  static_ip = "{{ .StaticIP }}"
}

source "hpe-vme-iso" "demo" {
  url                     = var.vme_url
  username                = var.vme_username
  password                = var.vme_password
  cluster_name            = "hpevmecluster"
  boot_command            = local.boot_command
  boot_wait               = "1s"
  http_interface          = "en0"
  http_directory          = "${path.root}/http"
  http_template_directory = "${path.root}/http_templates"
  http_port_min           = 8020
  http_port_max           = 8030
  vm_name                 = local.vm_name
  virtual_image_id        = 21
  group                   = "Platform Engineering"
  cloud                   = "HPE Demo"
  plan_id                 = 21
  ip_wait_timeout         = "25m"

  convert_to_template         = true
  template_name               = "rockytemplate_${local.template_version}"
  template_storage_bucket_id  = 1
  template_cloud_init_enabled = true
  template_labels             = ["rocky", "linux", local.patch_cycle]

  network_interface {
    network                   = "Compute"
    network_interface_type_id = 4
  }

  storage_volume {
    name            = "root"
    root_volume     = true
    size            = 25
    storage_type_id = 1
    datastore_id    = 17
  }

  storage_volume {
    name            = "data"
    root_volume     = false
    size            = 10
    storage_type_id = 1
    datastore_id    = 17
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
