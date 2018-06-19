# -*- mode: ruby -*-
# vi: set ft=ruby :

$script = <<SCRIPT
cat <<EOF> /etc/sysctl.d/99-sysctl.conf
fs.file-max=9999999
fs.nr_open=9999999
vm.overcommit_memory=1
vm.swappiness=0
net.core.somaxconn=65536
net.ipv4.tcp_max_syn_backlog=8192
EOF

cat <<EOF> /etc/security/limits.d/taskfit.conf
* hard nofile 999999
* soft nofile 999999
EOF

cat <<EOF> /etc/rc.local
#!/bin/sh -e
echo never > /sys/kernel/mm/transparent_hugepage/enabled
echo never > /sys/kernel/mm/transparent_hugepage/defrag
exit 0
EOF

curl -sSL https://get.docker.com/ |  sh
systemctl enable docker

docker run --name dama-redis -d -v /tmp:/tmp:rw taskfitio/dama-redis:latest
docker pull taskfitio/minimal:latest
mkdir -p /upload
docker run --name dama-server --net=host -d -v /tmp:/tmp:rw -v /upload:/upload -v /var/run/docker.sock:/var/run/docker.sock taskfitio/dama:latest
SCRIPT

Vagrant.configure(2) do |config|
  config.vm.box = "ubuntu/bionic64"
  config.vm.hostname = "dama-server"
  config.vm.network :private_network, ip: "10.20.1.15"
  config.vm.network "forwarded_port", guest: 8443, host: 8443, auto_correct: true

  config.vm.provider "virtualbox" do |v|
      v.name = "dama-server"
      v.memory = 2048
      v.cpus = 2
      v.customize ["modifyvm", :id, "--natdnsproxy1", "on"]
      v.customize ["modifyvm", :id, "--natdnshostresolver1", "on"]
  end
  config.vm.provision "shell", inline: $script, privileged: true
end