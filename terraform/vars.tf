variable "vpc_cidr" {
  type = string
  default = "10.0.0.0/16"
}

variable "az_suffixes" {
  type = list(string)
  default = [
    "a",
    "b",
  ]
}

variable "region" {
  type = string
  default = "us-west-2"
}

variable "az_number" {
  # Assign a number to each AZ letter used in our configuration
  default = {
    a = 1
    b = 2
    c = 3
    d = 4
    e = 5
    f = 6
  }
}
