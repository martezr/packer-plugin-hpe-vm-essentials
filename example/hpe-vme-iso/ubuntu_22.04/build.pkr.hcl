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
  vm_name   = "packiso-${local.timestamp}"
  boot_command = [
    "e<wait><down><down><down><end>",
    "ip={{ .StaticIP }}::{{ .StaticGateway }}:{{ .StaticMask }}:${local.vm_name}:::{{ .StaticDNS }} autoinstall 'ds=nocloud-net;s=http://{{ .HTTPIP }}:{{ .HTTPPort }}/'",
    "<wait><F10><wait>"
  ]
  static_ip = "{{ .StaticIP }}"
}

source "hpe-vme-iso" "demo" {
  url                 = var.vme_url
  username            = var.vme_username
  password            = var.vme_password
  cluster_name        = "vmecluster01"
  boot_command        = local.boot_command
  boot_wait           = "5s"
  convert_to_template = false
  skip_agent_install  = true
  vm_name             = local.vm_name
  template_name       = "packertest"
  virtual_image_id    = 1015
  group_id            = 3
  cloud_id            = 139
  plan_id             = 174

  http_interface          = "en0"
  http_directory          = "${path.root}/http"
  http_template_directory = "${path.root}/http_templates"
  http_port_max           = 8030
  http_port_min           = 8020

  convert_to_template         = true
  template_name               = "ubuntutemplate_${local.template_version}"
  template_storage_bucket_id  = 1
  template_cloud_init_enabled = true
  template_labels             = ["ubuntu", "linux", local.patch_cycle]


  network_interface {
    network_id                = 229
    network_interface_type_id = 4
  }

  storage_volume {
    name            = "root"
    root_volume     = true
    size            = 25
    storage_type_id = 1
    datastore_id    = 49
  }

  storage_volume {
    name            = "data"
    root_volume     = false
    size            = 5
    storage_type_id = 1
    datastore_id    = 15
  }

  # Raise the timeout, when installation takes longer
  ssh_timeout  = "55m"
  communicator = "ssh"
  ssh_username = "ubuntu"
  ssh_password = "password123"
}

build {
  sources = [
    "source.hpe-vme-iso.demo"
  ]

  provisioner "shell" {
    inline = ["echo foo"]
  }

  provisioner "shell" {
    inline = [
      "while [ ! -f /var/lib/cloud/instance/boot-finished ]; do echo 'Waiting for cloud-init...'; sleep 1; done",
      "sudo rm /etc/ssh/ssh_host_*",
      "sudo truncate -s 0 /etc/machine-id",
      "sudo apt -y autoremove --purge",
      "sudo apt -y clean",
      "sudo apt -y autoclean",
      "sudo cloud-init clean",
      "sudo rm -f /etc/cloud/cloud.cfg.d/subiquity-disable-cloudinit-networking.cfg",
      "sudo sync"
    ]
  }

  /*
  provisioner "mvm-morpheus" {
    url = var.morpheus_url
    username = var.morpheus_username
    password = var.morpheus_password
    task_id = 2
  }
  */
}
