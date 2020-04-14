FROM ipfs/testground-debian:buster

# https://github.com/docker/docker/blob/master/project/PACKAGERS.md#runtime-dependencies
RUN set -eux; \
	apt-get update --quiet; \
	apt-get install --no-install-recommends --yes \
		btrfs-progs \
		e2fsprogs \
		iproute2 \
		iptables \
		openssl \
		uidmap \
		xfsprogs \
		xz-utils \
# pigz: https://github.com/moby/moby/pull/35697 (faster gzip implementation)
		pigz \
	; \
# only install zfs if it's available for the current architecture
# https://packages.debian.org/source/sid/zfs-linux
# "apt-cache search" exits with a zero exit code but no output when the package
# exists but not for this arch
	if [ -n "$(apt-cache search --names-only zfsutils-linux)" ]; then \
		apt-get install --no-install-recommends --yes zfsutils-linux; \
	fi; \
	rm -rf /var/lib/apt

# TODO aufs-tools

# set up subuid/subgid so that "--userns-remap=default" works out-of-the-box
RUN set -x \
	&& addgroup --system dockremap \
	&& adduser --system --ingroup dockremap dockremap \
	&& echo 'dockremap:165536:65536' >> /etc/subuid \
	&& echo 'dockremap:165536:65536' >> /etc/subgid

# https://github.com/docker/docker/tree/master/hack/dind
ENV DIND_COMMIT 37498f009d8bf25fbb6199e8ccd34bed84f2874b

RUN set -eux; \
	wget --quiet -O /usr/local/bin/dind "https://raw.githubusercontent.com/docker/docker/${DIND_COMMIT}/hack/dind"; \
	chmod +x /usr/local/bin/dind

COPY dockerd-entrypoint.sh /usr/local/bin/

VOLUME /var/lib/docker
EXPOSE 2375 2376

ENTRYPOINT ["dockerd-entrypoint.sh"]
CMD []
