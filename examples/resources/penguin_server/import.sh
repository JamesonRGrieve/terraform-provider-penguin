# Import a server by its numeric id. Re-declare `environment` in config after
# import — the API does not return it in a form the resource tracks.
terraform import penguin_server.survival 12
