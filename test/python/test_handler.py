#
# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0
#
import nuclio_sdk

def handler_serve(context: nuclio_sdk.Context, event: nuclio_sdk.Event):
    context.logger.info_with('Invoked', method=event.method)
    return "Hello, from Nuclio :]"