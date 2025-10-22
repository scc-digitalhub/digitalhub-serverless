#
# SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0
#
import nuclio_sdk

def handler_serve(context: nuclio_sdk.Context, event: nuclio_sdk.Event):
    phase = event.headers.get('processing-phase').decode('utf-8')
    context.logger.info_with('Invoked processing phase', phase=phase)    
    # context.logger.info_with('Invoked method', method=event.method)
    # context.logger.info_with('Invoked path', path=event.path)
    context.logger.info_with('Invoked body', body=event.body.decode('utf-8'))
    # context.logger.info_with('Invoked headers', headers=event.headers)
    # request body processing phase
    if phase == '2':
        return event.body.decode('utf-8') + " - preprocessed"
    # response body processing phase
    if phase == '5':
        return event.body.decode('utf-8') + " - postprocessed"
    # default response
    return "Hello, from Nuclio :]"