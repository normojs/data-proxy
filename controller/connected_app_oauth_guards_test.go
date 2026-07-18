/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestEnsureConnectedAppOAuthAuthorizationClient(t *testing.T) {
	t.Parallel()

	t.Run("nil app", func(t *testing.T) {
		t.Parallel()
		err := ensureConnectedAppOAuthAuthorizationClient(nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid client_id")
	})

	t.Run("disabled", func(t *testing.T) {
		t.Parallel()
		app := &model.ConnectedApp{
			Status:            model.ConnectedAppStatusDisabled,
			AuthorizationFlow: model.ConnectedAppAuthorizationFlowAuthorizationCode,
		}
		err := ensureConnectedAppOAuthAuthorizationClient(app)
		require.Error(t, err)
		require.Contains(t, err.Error(), "client is disabled")
	})

	t.Run("device_code only", func(t *testing.T) {
		t.Parallel()
		app := &model.ConnectedApp{
			Status:            model.ConnectedAppStatusEnabled,
			AuthorizationFlow: model.ConnectedAppAuthorizationFlowDeviceCode,
		}
		err := ensureConnectedAppOAuthAuthorizationClient(app)
		require.Error(t, err)
		require.Contains(t, err.Error(), "authorization_code")
	})

	t.Run("authorization_code ok", func(t *testing.T) {
		t.Parallel()
		app := &model.ConnectedApp{
			Status:            model.ConnectedAppStatusEnabled,
			AuthorizationFlow: model.ConnectedAppAuthorizationFlowAuthorizationCode,
		}
		require.NoError(t, ensureConnectedAppOAuthAuthorizationClient(app))
	})

	t.Run("both ok", func(t *testing.T) {
		t.Parallel()
		app := &model.ConnectedApp{
			Status:            model.ConnectedAppStatusEnabled,
			AuthorizationFlow: model.ConnectedAppAuthorizationFlowBoth,
		}
		require.NoError(t, ensureConnectedAppOAuthAuthorizationClient(app))
	})
}
