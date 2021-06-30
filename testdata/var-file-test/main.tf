resource "random_id" "id" {
  count       = var.random_count
  byte_length = 16
}
