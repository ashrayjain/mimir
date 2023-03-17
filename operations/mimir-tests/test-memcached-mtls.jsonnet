local mimir = import 'mimir/mimir.libsonnet';

mimir {
  _config+:: {
    namespace: 'default',
    external_url: 'http://test',

    storage_backend: 'gcs',
    blocks_storage_bucket_name: 'blocks-bucket',

    ruler_enabled: true,
    ruler_storage_bucket_name: 'rules-bucket',

    alertmanager_enabled: true,
    alertmanager_storage_bucket_name: 'alerts-bucket',

    memcached_frontend_mtls_enabled: true,
    memcached_frontend_mtls_ca_cert_path: '/var/secrets/memcached/ca/memcached-ca-cert.pem',
    memcached_frontend_mtls_server_cert_path: '/var/secrets/memcached/server-cert/memcached-server-cert.pem',
    memcached_frontend_mtls_server_key_path: '/var/secrets/memcached/server-key/memcached-server-key.pem',
    memcached_frontend_mtls_client_cert_path: '/var/secrets/memcached/client-cert/memcached-client-cert.pem',
    memcached_frontend_mtls_client_key_path: '/var/secrets/memcached/client-key/memcached-client-key.pem',

    memcached_index_queries_mtls_enabled: true,
    memcached_index_queries_mtls_ca_cert_path: '/var/secrets/memcached/ca/memcached-ca-cert.pem',
    memcached_index_queries_mtls_server_cert_path: '/var/secrets/memcached/server-cert/memcached-server-cert.pem',
    memcached_index_queries_mtls_server_key_path: '/var/secrets/memcached/server-key/memcached-server-key.pem',
    memcached_index_queries_mtls_client_cert_path: '/var/secrets/memcached/client-cert/memcached-client-cert.pem',
    memcached_index_queries_mtls_client_key_path: '/var/secrets/memcached/client-key/memcached-client-key.pem',

    memcached_chunks_mtls_enabled: true,
    memcached_chunks_mtls_ca_cert_path: '/var/secrets/memcached/ca/memcached-ca-cert.pem',
    memcached_chunks_mtls_server_cert_path: '/var/secrets/memcached/server-cert/memcached-server-cert.pem',
    memcached_chunks_mtls_server_key_path: '/var/secrets/memcached/server-key/memcached-server-key.pem',
    memcached_chunks_mtls_client_cert_path: '/var/secrets/memcached/client-cert/memcached-client-cert.pem',
    memcached_chunks_mtls_client_key_path: '/var/secrets/memcached/client-key/memcached-client-key.pem',

    memcached_metadata_mtls_enabled: true,
    memcached_metadata_mtls_ca_cert_path: '/var/secrets/memcached/ca/memcached-ca-cert.pem',
    memcached_metadata_mtls_server_cert_path: '/var/secrets/memcached/server-cert/memcached-server-cert.pem',
    memcached_metadata_mtls_server_key_path: '/var/secrets/memcached/server-key/memcached-server-key.pem',
    memcached_metadata_mtls_client_cert_path: '/var/secrets/memcached/client-cert/memcached-client-cert.pem',
    memcached_metadata_mtls_client_key_path: '/var/secrets/memcached/client-key/memcached-client-key.pem',
  },
}
