use std::net::{Ipv4Addr, TcpListener, TcpStream};

const LISTENING_PORT: u16 = 1234;

#[async_std::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let mut sync_client = testground::sync::Client::new().await?;

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

            sync_client.signal("listening".to_string()).await?;

            for _stream in listener.incoming() {
                println!("Established inbound TCP connection.");
            }
        }
        std::net::IpAddr::V4(addr) if addr.octets()[3] == 3 => {
            println!("Test instance, connecting to listening instance.");

            sync_client
                .wait_for_barrier("listening".to_string(), 1)
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
            panic!("Unexpected local IP address {:?}", addr);
        }
    }

    Ok(())
}
