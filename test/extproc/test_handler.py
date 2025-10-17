#
# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0
#
import nuclio_sdk

def handler_serve(context: nuclio_sdk.Context, event: nuclio_sdk.Event):
    context.logger.info_with('Invoked processing phase', method=event.headers.get('processing-phase'))    
    context.logger.info_with('Invoked method', method=event.method)
    context.logger.info_with('Invoked path', path=event.path)
    context.logger.info_with('Invoked body', body=event.body)
    context.logger.info_with('Invoked headers', headers=event.headers)
    return "Hello, from Nuclio :]"