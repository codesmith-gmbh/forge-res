from pathlib import Path


def handler(event, _):
    request = event['Records'][0]['cf']['request']
    uri = request['uri']
    # We consider "file" names without dot to be folders
    if '.' not in Path(uri).name:
        # We ensure that '/index.hml' is at the end of the folder url
        suffix = 'index.html' if uri.endswith('/') else '/index.html'
        request['uri'] += suffix
    return request
