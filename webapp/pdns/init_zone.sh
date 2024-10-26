#!/usr/bin/env bash

set -eux
cd $(dirname $0)

if test -f /home/isucon/env.sh; then
	. /home/isucon/env.sh
fi

ISUCON_DB_HOST=${ISUCON13_MYSQL2_DIALCONFIG_ADDRESS:-127.0.0.1}
ISUCON_DB_PORT=${ISUCON13_MYSQL2_DIALCONFIG_PORT:-3306}
ISUCON_DB_USER=${ISUCON13_MYSQL2_DIALCONFIG_USER:-isucon}
ISUCON_DB_PASSWORD=${ISUCON13_MYSQL2_DIALCONFIG_PASSWORD:-isucon}
PDNS_DB_NAME=${ISUCON13_MYSQL2_DIALCONFIG_PASSWORD:-isudns}


# MySQLを初期化
mysql -u"$ISUCON_DB_USER" \
		-p"$ISUCON_DB_PASSWORD" \
		--host "$ISUCON_DB_HOST" \
		--port "$ISUCON_DB_PORT" \
		 < initdb.d/00_create_database.sql

mysql -u"$ISUCON_DB_USER" \
		-p"$ISUCON_DB_PASSWORD" \
		--host "$ISUCON_DB_HOST" \
		--port "$ISUCON_DB_PORT" \
		"$PDNS_DB_NAME" < initdb.d/10_schema.sql


ISUCON_SUBDOMAIN_ADDRESS=${ISUCON13_POWERDNS_SUBDOMAIN_ADDRESS:-127.0.0.1}

temp_dir=$(mktemp -d)
trap 'rm -rf $temp_dir' EXIT
sed 's/<ISUCON_SUBDOMAIN_ADDRESS>/'$ISUCON_SUBDOMAIN_ADDRESS'/g' u.isucon.local.zone > ${temp_dir}/u.isucon.local.zone
pdnsutil load-zone u.isucon.local ${temp_dir}/u.isucon.local.zone

