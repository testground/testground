use std::net::{Ipv4Addr, TcpListener, TcpStream};

const LISTENING_PORT: u16 = 1234;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let (client, _run_parameters) = testground::client::Client::new().await?;
    client.wait_network_initialized().await?;

    let local_addr = &if_addrs::get_if_addrs()
        .unwrap()
        .into_iter()
        .find(|iface| iface.name == "eth1")
        .unwrap()
        .addr
        .ip();

    match local_addr {
        std::net::IpAddr::V4(addr) if addr.octets()[3] == 2 => {
            println!("Test instance, listening for incoming connections.");

            let listener = TcpListener::bind((*addr, LISTENING_PORT))?;

            client.signal("listening".to_string()).await?;

            for _stream in listener.incoming() {
                println!("Established inbound TCP connection.");
                break;
            }
        }
        std::net::IpAddr::V4(addr) if addr.octets()[3] == 3 => {
            println!("Test instance, connecting to listening instance.");

            client
                .barrier("listening".to_string(), 1)
                .await?;

            let remote_addr: Ipv4Addr = {
                let mut octets = addr.octets();
                octets[3] = 2;
                octets.into()
            };
            let _stream = TcpStream::connect((remote_addr, LISTENING_PORT)).unwrap();
            println!("Established outbound TCP connection.");
        }
        addr => {
            client.record_failure("Unexpected local IP address")
                .await?;
            panic!("Unexpected local IP address {:?}", addr);
        }
    }

    client.record_success()
        .await?;
    println!("Done!");
    Ok(())
}
