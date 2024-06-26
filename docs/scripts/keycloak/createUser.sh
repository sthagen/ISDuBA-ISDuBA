#!/usr/bin/env bash

# This file is Free Software under the Apache-2.0 License
# without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
#
# SPDX-License-Identifier: Apache-2.0
#
# SPDX-FileCopyrightText: 2024 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
# Software-Engineering: 2024 Intevation GmbH <https://intevation.de>

set -e # to exit if a command in the script fails

# Usage: createUser.sh username first_name last_name email_address password [login]
# where:
#  username: username of the user
#  first_name: first name of the user
#  last_name: surname of the user
#  email_address: the users registrered email-address
#  password: password of the user
#  login=[true|false]: whether to login to keycloak again (default:true)

login=true
if [ ! -z "$6" ]; then
  case "$6" in
    true)
      # login is already true
      ;;
    false)
      login=false
      ;;
    *)
      echo "createUser.sh login can only be set to true or false"
      exit 1
      ;;
  esac
fi

if "$login"; then
  # This will work if the standard or custom has been set in env. If neither, this will fail.
  if [[ -z "${KEYCLOAK_ADMIN}" ]]; then
    export KEYCLOAK_ADMIN="keycloak"
    echo "No Keycloak admin set. Assuming admin with name \"keycloak\""
  else
    export KEYCLOAK_ADMIN="${KEYCLOAK_ADMIN}"
  fi

  if [[ -z "${KEYCLOAK_ADMIN_PASSWORD}" ]]; then
    export KEYCLOAK_ADMIN_PASSWORD="keycloak"
    echo "No Keycloak admin password set. Assuming admin with password \"keycloak\""
  else
    export KEYCLOAK_ADMIN_PASSWORD="${KEYCLOAK_ADMIN_PASSWORD}"
  fi

  sudo /opt/keycloak/bin/kcadm.sh config credentials --server http://localhost:8080 --realm master --user "$KEYCLOAK_ADMIN" --password "$KEYCLOAK_ADMIN_PASSWORD"
fi

if sudo /opt/keycloak/bin/kcadm.sh get 'http://localhost:8080/admin/realms/isduba/users' | grep -F -q "\"username\" : \"$1\"" ; then
  echo "User $1 already exists."
else
  # create user
  userid=$(sudo /opt/keycloak/bin/kcadm.sh create users --target-realm isduba \
      --set username=$1 --set enabled=true \
      --set firstName=$2 --set lastName=$3 \
      --set email=$4 \
      --set emailVerified=true)

  # set password for user
  sudo /opt/keycloak/bin/kcadm.sh set-password --target-realm isduba \
      --username $1 --new-password "$5"
fi
